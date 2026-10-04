//go:build !wasm

package dom

import "webtyp.com/fmt"

// domBackend is a stub implementation for non-WASM environments (e.g., SSR).
type domBackend struct {
	*tinyDOM
}

// newDom returns a new instance of the domBackend.
func newDom(td *tinyDOM) DOM {
	return &domBackend{
		tinyDOM: td,
	}
}

// Get retrieves an element by ID.
func (d *domBackend) Get(id string) (Reference, bool) {
	return &elementStub{}, true
}

// Render is not implemented for backend.
func (d *domBackend) Render(parentID string, component Component) error {
	return fmt.Errf("Render to parent is not supported on backend. Use String() directly on component.")
}

// Append is not implemented for backend.
func (d *domBackend) Append(parentID string, component Component) error {
	return fmt.Errf("Append not supported in backend/stub")
}

func (d *domBackend) OnHashChange(handler func(hash string)) {}

func (d *domBackend) OnScrollCapture(handler func(scrollTop float64)) {}

func (d *domBackend) OnUserActivity(handler func()) {}

// Show mounts the subtree returned by build only when cond is true.
// When false, the builder is not invoked and the container is empty.
func Show(cond *SignalBool, build func() *Element) *Element {
	container := NewElement("div")
	if cond.Get() {
		container.Child(build())
	}
	return container
}
func (d *domBackend) GetHash() string     { return "" }
func (d *domBackend) SetHash(hash string) {}

// elementStub is a no-op implementation of Reference for backend.
type elementStub struct{}

func (e *elementStub) GetAttr(key string) string                { return "" }
func (e *elementStub) Value() string                            { return "" }
func (e *elementStub) SetValue(value string)                    {}
func (e *elementStub) SetAttr(key, value string)                {}
func (e *elementStub) RemoveAttr(key string)                    {}
func (e *elementStub) SetText(text string)                      {}
func (e *elementStub) Checked() bool                            { return false }
func (e *elementStub) OnClick(handler func(event Event))        {}
func (e *elementStub) OnChange(handler func(event Event))       {}
func (e *elementStub) OnInput(handler func(event Event))        {}
func (e *elementStub) OnBlur(handler func(event Event))         {}
func (e *elementStub) OnSubmit(handler func(event Event))       {}
func (e *elementStub) OnToggle(handler func(event Event))       {}
func (e *elementStub) OnMouseEnter(handler func(event Event))   {}
func (e *elementStub) OnMouseLeave(handler func(event Event))   {}
func (e *elementStub) OnMouseDown(handler func(event Event))    {}
func (e *elementStub) OnMouseUp(handler func(event Event))      {}
func (e *elementStub) OnPointerDown(handler func(event Event))  {}
func (e *elementStub) OnPointerUp(handler func(event Event))    {}
func (e *elementStub) OnPointerEnter(handler func(event Event)) {}
func (e *elementStub) OnPointerLeave(handler func(event Event)) {}
func (e *elementStub) OnPointerMove(handler func(event Event))  {}
func (e *elementStub) OnTouchStart(handler func(event Event))   {}
func (e *elementStub) OnTouchMove(handler func(event Event))    {}
func (e *elementStub) OnTouchEnd(handler func(event Event))     {}
func (e *elementStub) OnFocusIn(handler func(event Event))      {}
func (e *elementStub) OnFocusOut(handler func(event Event))     {}
func (e *elementStub) OnKeyDown(handler func(event KeyEvent))   {}
func (e *elementStub) Focus()                                   {}
func (e *elementStub) ScrollIntoView()                          {}
func (e *elementStub) ScrollIntoViewInstant()                   {}
func (e *elementStub) ScrollsX() bool                           { return false }

// eventStub is the backend Event: the backend never fires events, so every
// accessor reads as absent — Key() returns "" like every other missing value
// in this package ("-" = absent convention).
type eventStub struct{}

func (e *eventStub) PreventDefault()        {}
func (e *eventStub) StopPropagation()       {}
func (e *eventStub) TargetValue() string    { return "" }
func (e *eventStub) TargetID() string       { return "" }
func (e *eventStub) TargetChecked() bool    { return false }
func (e *eventStub) Buttons() int           { return 0 }
func (e *eventStub) ReleasePointerCapture() {}
func (e *eventStub) Key() Key               { return "" }
