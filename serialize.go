package dom

import (
	"webtyp.com/fmt"
)

type childRenderer func(Component) string

type elementObserver func(*Element)

func updateForAttr(el *Element, oldID, newID string) {
	if el == nil || oldID == "" || newID == "" {
		return
	}
	for i, attr := range el.attrs {
		if attr.Key == "for" && attr.Value == oldID {
			el.attrs[i].Value = newID
		}
	}
	for _, child := range el.children {
		if childEl, ok := child.(*Element); ok {
			updateForAttr(childEl, oldID, newID)
		}
	}
}

func sanitizeChildIDs(el *Element) {
	if el == nil {
		return
	}
	for _, child := range el.children {
		if childEl, ok := child.(*Element); ok {
			if childEl.id != "" {
				oldID := childEl.id
				if childEl.key == "" {
					childEl.key = oldID
				}
				newID := generateID()
				childEl.id = newID
				updateForAttr(el, oldID, newID)
			}
			sanitizeChildIDs(childEl)
		}
	}
}

// serializeElement renders an Element tree into an HTML string.
// ownerID identifies the component instance owning this tree (empty if top-level integration element).
// isRoot is true if el is the root element of component ownerID.
// renderChild resolves Component children to HTML (SSR vs WASM lifecycle tracking).
// observer is an optional callback invoked for each Element in the tree (used by WASM to collect pending events).
func serializeElement(el *Element, renderChild childRenderer, ownerID string, isRoot bool, observer ...elementObserver) string {
	if el == nil {
		return ""
	}
	beginPass()
	defer endPass()

	var obs elementObserver
	hasObserver := len(observer) > 0 && observer[0] != nil

	hasOwner := ownerID != ""
	if hasOwner && isRoot {
		el.id = ownerID
		sanitizeChildIDs(el)
	}

	if hasObserver {
		if (len(el.events) > 0 || len(el.bindings) > 0 || el.autofocus) && el.id == "" {
			el.id = generateID()
		}
		obs = observer[0]
		obs(el)
	}

	s := "<" + el.tag
	if el.id != "" {
		claimID(el.id, el.tag)
		s += " id='" + fmt.Convert(el.id).EscapeAttr() + "'"
	}

	classes := el.classes
	attrs := el.attrs

	if el.key != "" {
		hasDataKey := false
		for _, a := range attrs {
			if a.Key == "data-key" {
				hasDataKey = true
				break
			}
		}
		if !hasDataKey {
			attrs = append(attrs, fmt.KeyValue{Key: "data-key", Value: el.key})
		}
	}

	textContent := ""
	hasTextContent := false
	var boundChildren []*Element

	for _, b := range el.bindings {
		switch b.kind {
		case "text":
			if b.signal != nil {
				if sig, ok := b.signal.(*SignalString); ok {
					textContent = sig.Get()
				}
			} else if b.fnString != nil {
				textContent = b.fnString()
			}
			hasTextContent = true
		case "attr":
			val := ""
			if b.signal != nil {
				if sig, ok := b.signal.(*SignalString); ok {
					val = sig.Get()
				}
			} else if b.fnString != nil {
				val = b.fnString()
			}
			found := false
			for i, attr := range attrs {
				if attr.Key == b.name {
					attrs[i].Value = val
					found = true
					break
				}
			}
			if !found {
				attrs = append(attrs, fmt.KeyValue{Key: b.name, Value: val})
			}
		case "class":
			on := false
			if b.signal != nil {
				if sig, ok := b.signal.(*SignalBool); ok {
					on = sig.Get()
				}
			} else if b.fnBool != nil {
				on = b.fnBool()
			}
			if on {
				classes = append(classes, b.name)
			}
		case "attrbool":
			on := false
			if b.signal != nil {
				if sig, ok := b.signal.(*SignalBool); ok {
					on = sig.Get()
				}
			} else if b.fnBool != nil {
				on = b.fnBool()
			}
			if on {
				attrs = append(attrs, fmt.KeyValue{Key: b.name, Value: ""})
			}
		case "state":
			on := false
			if b.signal != nil {
				if sig, ok := b.signal.(*SignalBool); ok {
					on = sig.Get()
				}
			} else if b.fnBool != nil {
				on = b.fnBool()
			}
			if on {
				attrs = append(attrs, fmt.KeyValue{Key: b.state.Key(), Value: b.state.Value()})
			}
		case "value":
			val := ""
			if b.signal != nil {
				if sig, ok := b.signal.(*SignalString); ok {
					val = sig.Get()
				}
			}
			attrs = append(attrs, fmt.KeyValue{Key: "value", Value: val})
		case "children":
			// WASM-only, same reasoning as the id auto-generation above: a
			// SignalNodes drives reactive updates after the client hydrates,
			// it has no bearing on SSR. Before unifying the two serializers,
			// elementToHTML (SSR) never had a "children" case at all — an
			// element seeding static rows via Child(...) AND wiring
			// BindChildren for later reactivity (the pattern datatable uses)
			// rendered only the static rows on SSR. Processing this
			// unconditionally made SSR render the signal's current nodes on
			// top of the static ones, double-emitting every row and
			// panicking claimID on the resulting duplicate ids.
			if hasObserver {
				if sig, ok := b.signal.(*SignalNodes); ok {
					boundChildren = append(boundChildren, sig.Get()...)
				}
			}
		}
	}

	if len(classes) > 0 {
		s += " class='"
		for i, c := range classes {
			if i > 0 {
				s += " "
			}
			s += fmt.Convert(c).EscapeAttr()
		}
		s += "'"
	}
	for _, attr := range attrs {
		s += " " + fmt.Convert(attr.Key).EscapeAttr() + "='" + fmt.Convert(attr.Value).EscapeAttr() + "'"
	}
	s += ">"
	if el.void {
		return s
	}

	if hasTextContent {
		s += fmt.Convert(textContent).EscapeHTML()
	} else {
		for _, node := range boundChildren {
				s += serializeElement(node, renderChild, ownerID, false, obs)
		}
		for _, child := range el.children {
			switch v := child.(type) {
			case interface {
				Component
				ViewRenderer
			}:
				s += renderChild(v)
			case *Element:
					s += serializeElement(v, renderChild, ownerID, false, obs)
			case TrustedHTML:
				s += string(v)
			case string:
				s += fmt.Convert(v).EscapeHTML()
			case Component:
				s += renderChild(v)
			default:
				s += fmt.Convert(fmt.Sprint(v)).EscapeHTML()
			}
		}
	}
	s += "</" + el.tag + ">"
	return s
}
