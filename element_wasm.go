//go:build wasm

package dom

import (
	"syscall/js"
)

// elementWasm is the WASM implementation of the Reference interface.
type elementWasm struct {
	val js.Value
	dom *domWasm
	id  string
}

// GetAttr retrieves an attribute value.
func (e *elementWasm) GetAttr(key string) string {
	return e.val.Call("getAttribute", key).String()
}

// Value returns the current value of an input/textarea/select.
func (e *elementWasm) Value() string {
	return e.val.Get("value").String()
}

// SetValue sets element.value.
func (e *elementWasm) SetValue(value string) {
	e.val.Set("value", value)
}

// SetAttr calls element.setAttribute.
func (e *elementWasm) SetAttr(key, value string) {
	e.val.Call("setAttribute", key, value)
}

// RemoveAttr calls element.removeAttribute.
func (e *elementWasm) RemoveAttr(key string) {
	e.val.Call("removeAttribute", key)
}

// SetText sets element.textContent.
func (e *elementWasm) SetText(text string) {
	e.val.Set("textContent", text)
}

// Checked returns current checked state.
func (e *elementWasm) Checked() bool {
	return e.val.Get("checked").Bool()
}

// on registers a handler for the literal event type. Unexported: the engine
// dispatches stored handler names through it, and every other caller uses a
// typed On* method — the type is never a free string outside this file.
func (e *elementWasm) on(eventType string, handler func(event Event)) {
	eventKey := e.id + "::" + eventType
	fn := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		evt := eventWasm{Value: args[0]}
		handler(&evt)
		return nil
	})
	e.val.Call("addEventListener", eventType, fn)

	// Append to eventFuncs
	e.dom.eventFuncs = append(e.dom.eventFuncs, struct {
		key string
		val js.Value
		fn  js.Func
	}{eventKey, e.val, fn})

	// Associate the event with the component currently being mounted.
	if e.dom.currentComponentID != "" {
		compID := e.dom.currentComponentID
		found := false
		for i, item := range e.dom.componentListeners {
			if item.id == compID {
				e.dom.componentListeners[i].keys = append(e.dom.componentListeners[i].keys, eventKey)
				found = true
				break
			}
		}
		if !found {
			e.dom.componentListeners = append(e.dom.componentListeners, struct {
				id   string
				keys []string
			}{compID, []string{eventKey}})
		}
	}
}

// OnClick registers a click handler on the live node.
func (e *elementWasm) OnClick(handler func(event Event)) {
	e.on("click", handler)
}

// OnChange registers a change handler on the live node.
func (e *elementWasm) OnChange(handler func(event Event)) {
	e.on("change", handler)
}

// OnInput registers an input handler on the live node.
func (e *elementWasm) OnInput(handler func(event Event)) {
	e.on("input", handler)
}

// OnBlur registers a blur handler on the live node.
func (e *elementWasm) OnBlur(handler func(event Event)) {
	e.on("blur", handler)
}

// OnSubmit registers a submit handler on the live node.
func (e *elementWasm) OnSubmit(handler func(event Event)) {
	e.on("submit", handler)
}

// OnToggle registers a toggle handler on the live node.
func (e *elementWasm) OnToggle(handler func(event Event)) {
	e.on("toggle", handler)
}

// OnMouseEnter registers a mouseenter handler on the live node.
func (e *elementWasm) OnMouseEnter(handler func(event Event)) {
	e.on("mouseenter", handler)
}

// OnMouseLeave registers a mouseleave handler on the live node.
func (e *elementWasm) OnMouseLeave(handler func(event Event)) {
	e.on("mouseleave", handler)
}

// OnFocusIn registers a focusin handler on the live node.
func (e *elementWasm) OnFocusIn(handler func(event Event)) {
	e.on("focusin", handler)
}

// OnFocusOut registers a focusout handler on the live node.
func (e *elementWasm) OnFocusOut(handler func(event Event)) {
	e.on("focusout", handler)
}

// OnKeyDown registers a keydown handler on the live node. The handler
// receives a KeyEvent: eventWasm implements Key(), so the assertion holds
// for every event the browser delivers here.
func (e *elementWasm) OnKeyDown(handler func(event KeyEvent)) {
	e.on("keydown", func(ev Event) {
		handler(ev.(KeyEvent))
	})
}

// Focus sets focus to the element.
//
// preventScroll is part of the contract, not a tweak: focus() otherwise makes
// the browser scroll EVERY scrollable ancestor to reveal the element, and here
// the layout owns scroll position, not the browser — ScrollIntoView below is
// the explicit way to move it. The two fight. On iOS Safari the browser's
// version wins and jumps a scroll-snap strip instantly, which both kills the
// smooth scroll the layout asked for and, once the keyboard is up, can leave
// the strip parked where it was with the caret on an off-screen field.
//
// Nothing depends on the suppressed behavior: every Focus() caller either
// targets an element that is already on screen (autofocus at mount, a search
// input inside a dropdown that just opened) or scrolls its own container
// itself. Keyboard opening is unaffected — that follows from focus() being
// synchronous with the user gesture, not from the scroll. Browsers without
// preventScroll ignore the options object and behave exactly as before.
func (e *elementWasm) Focus() {
	opts := e.dom.objectCtor.New()
	opts.Set("preventScroll", true)
	e.val.Call("focus", opts)
}

// ScrollsX reports whether the element's content overflows its box along the
// inline axis. The 1px slack absorbs sub-pixel layout rounding, which otherwise
// reports a strip as scrollable when it is exactly full.
func (e *elementWasm) ScrollsX() bool {
	return e.val.Get("scrollWidth").Float() > e.val.Get("clientWidth").Float()+1
}

// scrollOptions builds the options object scrollIntoView takes, off the
// singleton's cached Object constructor (domWasm.objectCtor) rather than a
// fresh js.Global() lookup — same reason document/localStorage are cached
// there. behavior is the only axis that ever varies between callers
// ("smooth" vs "instant"); inline/block stay fixed. No map — dom/AGENTS.md,
// "Slices Over Maps": three explicit Set calls instead.
func (e *elementWasm) scrollOptions(behavior string) js.Value {
	opts := e.dom.objectCtor.New()
	opts.Set("behavior", behavior)
	opts.Set("inline", "start")
	opts.Set("block", "nearest")
	return opts
}

// ScrollIntoView smooth-scrolls the element into view.
func (e *elementWasm) ScrollIntoView() {
	e.val.Call("scrollIntoView", e.scrollOptions("smooth"))
}

// ScrollIntoViewInstant jumps the element into view with no animation — an
// explicit "instant", not "auto": "auto" defers to the container's own
// scroll-behavior CSS, and callers use this method specifically to bypass
// that, unconditionally. e.g. a circular scroll-snap strip wrapping from its
// last panel back to its first, where a smooth scroll would visibly travel
// across every panel in between in the wrong apparent direction. Every
// other navigation should keep using ScrollIntoView; reach for this one
// only at the wrap boundary.
func (e *elementWasm) ScrollIntoViewInstant() {
	e.val.Call("scrollIntoView", e.scrollOptions("instant"))
}
