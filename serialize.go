package dom

import (
	"webtyp.com/fmt"
)

type childRenderer func(Component) string

type elementObserver func(*Element)

// serializeElement renders an Element tree into an HTML string.
// renderChild resolves Component children to HTML (SSR vs WASM lifecycle tracking).
// observer is an optional callback invoked for each Element in the tree (used by WASM to collect pending events).
func serializeElement(el *Element, renderChild childRenderer, observer ...elementObserver) string {
	if el == nil {
		return ""
	}
	beginPass()
	defer endPass()

	var obs elementObserver
	if len(observer) > 0 {
		obs = observer[0]
	}

	// One fmt.Builder for the whole tree, not one string per node glued
	// together with +=: string is immutable, so s += x reallocates and
	// recopies everything s already holds, every time — for a tree with N
	// bytes of total output that is O(N²) copying, not O(N), because every
	// parent re-copies the already-serialized text of all its children each
	// time it appends the next one. fmt.Builder (this package's answer to
	// strings.Builder — dom/AGENTS.md bans the stdlib one) grows one buffer
	// amortized and pays for the copy once.
	b := fmt.Convert()
	writeElement(b, el, renderChild, obs)
	return b.String()
}

// writeElement appends el's HTML to b. obs is nil on the SSR path and
// non-nil on WASM (see serializeElement) — hasObserver below is exactly the
// old "was an observer passed" check, just derived from a plain nil check
// now that obs is no longer a variadic slice at this layer.
func writeElement(b *fmt.Builder, el *Element, renderChild childRenderer, obs elementObserver) {
	if el == nil {
		return
	}

	hasObserver := obs != nil

	// Auto-generating an id for a bound/eventful element only matters where
	// something will later look that id up in a live DOM to patch or wire it
	// — the WASM path, signaled by the presence of an observer (it collects
	// pending events for exactly that purpose). SSR has no live DOM: gating
	// this on hasObserver keeps it out of that path, where it used to leak
	// into elementToHTML/(*Element).String() after unifying the two
	// serializers, minting a fresh id on every render of the same tree and
	// making SSR output for any bound element non-deterministic.
	//
	// A KEYED element joins that list for the same reason: Key is the author's
	// identity contract, and (*Element).Ref resolves the live node through the
	// id dom put on it here. Without an id a keyed element is unreachable and
	// Ref would answer a silent false. Still inside the hasObserver gate — SSR
	// emits no id for a key, so (*Element).String stays byte-identical across
	// renders.
	if hasObserver {
		if (len(el.events) > 0 || len(el.bindings) > 0 || el.autofocus || el.key != "") && el.id == "" {
			el.id = generateID()
		}
		obs(el)
	}

	b.WriteString("<").WriteString(el.tag)
	if el.id != "" {
		claimID(el.id, el.tag)
		b.WriteString(" id='").WriteString(fmt.Convert(el.id).EscapeAttr()).WriteString("'")
	}

	classes := el.classes
	attrs := el.attrs
	textContent := ""
	hasTextContent := false
	var boundChildren []*Element

	for _, bind := range el.bindings {
		switch bind.kind {
		case "text":
			if bind.signal != nil {
				if sig, ok := bind.signal.(*SignalString); ok {
					textContent = sig.Get()
				}
			} else if bind.fnString != nil {
				textContent = bind.fnString()
			}
			hasTextContent = true
		case "attr":
			val := ""
			if bind.signal != nil {
				if sig, ok := bind.signal.(*SignalString); ok {
					val = sig.Get()
				}
			} else if bind.fnString != nil {
				val = bind.fnString()
			}
			found := false
			for i, attr := range attrs {
				if attr.Key == bind.name {
					attrs[i].Value = val
					found = true
					break
				}
			}
			if !found {
				attrs = append(attrs, fmt.KeyValue{Key: bind.name, Value: val})
			}
		case "class":
			on := false
			if bind.signal != nil {
				if sig, ok := bind.signal.(*SignalBool); ok {
					on = sig.Get()
				}
			} else if bind.fnBool != nil {
				on = bind.fnBool()
			}
			if on {
				classes = append(classes, bind.name)
			}
		case "attrbool":
			on := false
			if bind.signal != nil {
				if sig, ok := bind.signal.(*SignalBool); ok {
					on = sig.Get()
				}
			} else if bind.fnBool != nil {
				on = bind.fnBool()
			}
			if on {
				attrs = append(attrs, fmt.KeyValue{Key: bind.name, Value: ""})
			}
		case "state":
			on := false
			if bind.signal != nil {
				if sig, ok := bind.signal.(*SignalBool); ok {
					on = sig.Get()
				}
			} else if bind.fnBool != nil {
				on = bind.fnBool()
			}
			if on {
				attrs = append(attrs, fmt.KeyValue{Key: bind.state.Key(), Value: bind.state.Value()})
			}
		case "value":
			val := ""
			if bind.signal != nil {
				if sig, ok := bind.signal.(*SignalString); ok {
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
				if sig, ok := bind.signal.(*SignalNodes); ok {
					boundChildren = append(boundChildren, sig.Get()...)
				}
			}
		}
	}

	if len(classes) > 0 {
		b.WriteString(" class='")
		for i, c := range classes {
			if i > 0 {
				b.WriteString(" ")
			}
			b.WriteString(fmt.Convert(c).EscapeAttr())
		}
		b.WriteString("'")
	}
	for _, attr := range attrs {
		b.WriteString(" ").WriteString(fmt.Convert(attr.Key).EscapeAttr()).
			WriteString("='").WriteString(fmt.Convert(attr.Value).EscapeAttr()).WriteString("'")
	}
	b.WriteString(">")
	if el.void {
		return
	}

	if hasTextContent {
		b.WriteString(fmt.Convert(textContent).EscapeHTML())
	} else {
		for _, node := range boundChildren {
			writeElement(b, node, renderChild, obs)
		}
		for _, child := range el.children {
			switch v := child.(type) {
			case *Element:
				writeElement(b, v, renderChild, obs)
			case TrustedHTML:
				b.WriteString(string(v))
			case string:
				b.WriteString(fmt.Convert(v).EscapeHTML())
			case Component:
				b.WriteString(renderChild(v))
			default:
				b.WriteString(fmt.Convert(fmt.Sprint(v)).EscapeHTML())
			}
		}
	}
	b.WriteString("</").WriteString(el.tag).WriteString(">")
}
