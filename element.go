package dom

import (
	"webtyp.com/fmt"
)

// Element represents a DOM element in the fluent Element API.
type Element struct {
	tag       string
	id        string
	key       string
	classes   []string
	attrs     []fmt.KeyValue
	events    []eventHandler
	bindings  []binding
	children  []any
	void      bool
	autofocus bool

	// attached reports that this element is already somebody's child. An
	// element has exactly one parent: ids are minted per element, so the same
	// pointer under two parents renders twice with ONE id, and every handler
	// and binding wires to whichever copy the runtime resolved — the other is
	// inert. A consumer that needs the same thing in two places has to build
	// two, usually through a factory.
	attached bool
}

type binding struct {
	kind     string // "text", "attr", "class", "attrbool", "state", "value", "children"
	name     string // attr name or class name
	state    StateAttr
	signal   subscribable
	fnString func() string
	fnBool   func() bool
}

// StateAttr is anything that names a data-state attribute and the value the
// stylesheet selects on. widget.State satisfies it; nothing else needs to.
//
// Declared here rather than imported so that dom keeps no dependency on the widget
// vocabulary — the same seam Class.AsAttr already uses in the other direction.
type StateAttr interface {
	Key() string
	Value() string
}

// NewElement creates an Element with the given HTML tag.
// Used by webtyp/html, webtyp/svg, webtyp/image to build elements.
func NewElement(tag string) *Element { return &Element{tag: tag} }

// NoCloseTag marks the element as self-closing (no closing tag rendered).
// Use for void HTML elements: br, hr, img, input, link, meta, etc.
func (b *Element) NoCloseTag() *Element {
	b.void = true
	return b
}

// ID sets the ID of the element.
func (b *Element) ID(id string) *Element {
	b.id = id
	return b
}

// For sets the for= attribute pointing to other's ID, auto-generating
// other's ID if it has none. Use for label/input pairing and aria-* references.
func (b *Element) For(other *Element) *Element {
	if other == nil {
		return b
	}
	return b.Attr("for", other.GetID())
}

// Key sets a stable identity for keyed reconciliation in BindChildren.
func (b *Element) Key(key string) *Element {
	b.key = key
	return b
}

// Autofocus marks the element to be focused when it first appears.
func (b *Element) Autofocus() *Element {
	b.autofocus = true
	return b
}

// Class adds a class to the element.
func (b *Element) Class(class ...string) *Element {
	b.classes = append(b.classes, class...)
	return b
}

// Attr sets an attribute on the element.
func (b *Element) Attr(key, val string) *Element {
	for i, attr := range b.attrs {
		if attr.Key == key {
			b.attrs[i].Value = val
			return b
		}
	}
	b.attrs = append(b.attrs, fmt.KeyValue{Key: key, Value: val})
	return b
}

// on registers a handler for the literal event type t. Unexported: the type
// is never a free string at a call site — each On* method below carries its
// own literal, so a mistyped type string cannot compile into a handler that
// never fires and reports nothing. The registration machinery is unchanged;
// only the door is typed.
func (b *Element) on(t string, h func(Event)) *Element {
	b.events = append(b.events, eventHandler{Name: t, Handler: h})
	return b
}

// OnClick registers a click handler.
func (b *Element) OnClick(h func(Event)) *Element {
	return b.on("click", h)
}

// OnChange registers a change handler.
func (b *Element) OnChange(h func(Event)) *Element {
	return b.on("change", h)
}

// OnInput registers an input handler.
func (b *Element) OnInput(h func(Event)) *Element {
	return b.on("input", h)
}

// OnBlur registers a blur handler.
func (b *Element) OnBlur(h func(Event)) *Element {
	return b.on("blur", h)
}

// OnSubmit registers a submit handler.
func (b *Element) OnSubmit(h func(Event)) *Element {
	return b.on("submit", h)
}

// OnToggle registers a toggle handler.
func (b *Element) OnToggle(h func(Event)) *Element {
	return b.on("toggle", h)
}

// OnMouseEnter registers a mouseenter handler.
func (b *Element) OnMouseEnter(h func(Event)) *Element {
	return b.on("mouseenter", h)
}

// OnMouseLeave registers a mouseleave handler.
func (b *Element) OnMouseLeave(h func(Event)) *Element {
	return b.on("mouseleave", h)
}

// OnFocusIn registers a focusin handler.
func (b *Element) OnFocusIn(h func(Event)) *Element {
	return b.on("focusin", h)
}

// OnFocusOut registers a focusout handler.
func (b *Element) OnFocusOut(h func(Event)) *Element {
	return b.on("focusout", h)
}

// OnKeyDown registers a keydown handler. The handler receives a KeyEvent —
// narrowed by the event — so the pressed key reads as a typed dom.Key:
//
//	el.OnKeyDown(func(e dom.KeyEvent) {
//		if e.Key() == dom.KeyArrowLeft { ... }
//	})
func (b *Element) OnKeyDown(h func(KeyEvent)) *Element {
	return b.on("keydown", func(e Event) {
		h(e.(KeyEvent))
	})
}

// Child adds one or more elements or components as children.
func (b *Element) Child(c ...Component) *Element {
	for _, child := range c {
		if child == nil {
			continue
		}
		if el, ok := child.(*Element); ok {
			if el.attached {
				panic(fmt.Err("dom: element", el.tag, el.id,
					"is already a child of another element; one element has one parent —",
					"build a second instance instead of sharing this one"))
			}
			el.attached = true
		}
		b.children = append(b.children, child)
	}
	return b
}

// Set applies multiple attributes or classes at once using KeyValue pairs.
func (b *Element) Set(kv ...fmt.KeyValue) *Element {
	for _, attr := range kv {
		switch attr.Key {
		case "class":
			b.Class(attr.Value)
		case "id":
			b.ID(attr.Value)
		default:
			b.Attr(attr.Key, attr.Value)
		}
	}
	return b
}

// Text adds a text node child.
func (b *Element) Text(text string) *Element {
	b.children = append(b.children, text)
	return b
}

// Raw agrega marcado sin escapar. Exige un TrustedHTML, así que pasar datos
// de una petición no compila — ver Trust.
func (b *Element) Raw(h TrustedHTML) *Element {
	b.children = append(b.children, h)
	return b
}

// BindText links the element's textContent to a SignalString.
func (b *Element) BindText(s *SignalString) *Element {
	b.bindings = append(b.bindings, binding{kind: "text", signal: s})
	return b
}

// BindAttr links an attribute to a SignalString.
func (b *Element) BindAttr(name string, s *SignalString) *Element {
	b.bindings = append(b.bindings, binding{kind: "attr", name: name, signal: s})
	return b
}

// BindClass toggles a class based on a SignalBool.
func (b *Element) BindClass(class string, on *SignalBool) *Element {
	b.bindings = append(b.bindings, binding{kind: "class", name: class, signal: on})
	return b
}

// BindAttrBool toggles a boolean attribute (disabled, checked, etc.) based on a SignalBool.
func (b *Element) BindAttrBool(name string, on *SignalBool) *Element {
	b.bindings = append(b.bindings, binding{kind: "attrbool", name: name, signal: on})
	return b
}

// Bind provides two-way binding for <input> and <textarea>.
func (b *Element) Bind(s *SignalString) *Element {
	b.bindings = append(b.bindings, binding{kind: "value", signal: s})
	return b
}

// BindChildren links a container's children to a SignalNodes.
func (b *Element) BindChildren(s *SignalNodes) *Element {
	b.bindings = append(b.bindings, binding{kind: "children", signal: s})
	return b
}

// BindTextFunc links the element's textContent to a computed string.
func (b *Element) BindTextFunc(fn func() string) *Element {
	b.bindings = append(b.bindings, binding{kind: "text", fnString: fn})
	return b
}

// BindAttrFunc links an attribute to a computed string.
func (b *Element) BindAttrFunc(name string, fn func() string) *Element {
	b.bindings = append(b.bindings, binding{kind: "attr", name: name, fnString: fn})
	return b
}

// BindClassFunc toggles a class based on a computed boolean.
func (b *Element) BindClassFunc(class string, fn func() bool) *Element {
	b.bindings = append(b.bindings, binding{kind: "class", name: class, fnBool: fn})
	return b
}

// BindAttrBoolFunc toggles a boolean attribute based on a computed boolean.
func (b *Element) BindAttrBoolFunc(name string, fn func() bool) *Element {
	b.bindings = append(b.bindings, binding{kind: "attrbool", name: name, fnBool: fn})
	return b
}

// BindState writes the state's attribute while on is true and removes it when
// false. This is the ONLY supported way to write a widget state: the value the
// stylesheet selects on comes from the state itself, so markup and CSS cannot
// disagree.
//
// Not BindAttrBool: that writes the HTML boolean form (`data-x=""`), which no
// data-state selector matches. That mistake shipped once and was invisible.
func (b *Element) BindState(s StateAttr, on *SignalBool) *Element {
	b.bindings = append(b.bindings, binding{kind: "state", state: s, signal: on})
	return b
}

// BindStateFunc is the computed form, for a state derived from more than one signal.
func (b *Element) BindStateFunc(s StateAttr, fn func() bool) *Element {
	b.bindings = append(b.bindings, binding{kind: "state", state: s, fnBool: fn})
	return b
}

// SetState writes the state unconditionally, for markup that is born in it.
func (b *Element) SetState(s StateAttr) *Element {
	return b.Attr(s.Key(), s.Value())
}

// Render renders the element to the parent.
// This is a terminal operation.
func (b *Element) Render(parentID string) error {
	return Render(parentID, b)
}

// --- Component Interface Implementation ---

// Ref returns the live DOM node this element was rendered into.
//
// It is the typed alternative to inventing a global id and calling Get on it:
// the author keeps the *Element they built and asks it for its node, so no name
// is chosen, and two instances of one component cannot collide.
//
//	func (c *Comp) Render() *Element {
//		c.row = NewElement("span").Key("row")
//		return NewElement("div").Child(c.row)
//	}
//	func (c *Comp) onSomething() {
//		if row, ok := c.row.Ref(); ok { row.SetText("hi") }
//	}
//
// ok is false before the element has been rendered, and for an element dom
// never gave an id — give it a Key to make it addressable. On the backend
// (SSR) there is no live DOM and Get's stub answer is returned unchanged.
func (b *Element) Ref() (Reference, bool) {
	if b.id == "" {
		return nil, false
	}
	return Get(b.id)
}

// GetID returns the element's ID.
func (b *Element) GetID() string {
	if b.id == "" {
		b.id = generateID()
	}
	return b.id
}

// SetID sets the element's ID.
func (b *Element) SetID(id string) {
	b.id = id
}

// String serializes the element tree to its string representation.
func (b *Element) String() string {
	return elementToHTML(b)
}

// Children returns the component's children (components only).
func (b *Element) Children() []Component {
	var comps []Component
	for _, child := range b.children {
		if c, ok := child.(Component); ok {
			comps = append(comps, c)
		}
	}
	return comps
}

// Helper to convert Element to HTML string (recursive)
func elementToHTML(el *Element) string {
	return serializeElement(el, func(c Component) string {
		if c == nil {
			return ""
		}
		return c.String()
	})
}
