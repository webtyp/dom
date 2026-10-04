//go:build wasm

package dom

import (
	"syscall/js"
)

// eventWasm is the WASM implementation of the Event interface.
type eventWasm struct {
	js.Value
}

// PreventDefault prevents the default action of the event.
func (e *eventWasm) PreventDefault() {
	e.Call("preventDefault")
}

// StopPropagation stops the event from bubbling up the DOM tree.
func (e *eventWasm) StopPropagation() {
	e.Call("stopPropagation")
}

// TargetValue returns the value of the event's target element.
func (e *eventWasm) TargetValue() string {
	v := e.Get("target").Get("value")
	if v.IsUndefined() || v.IsNull() {
		return ""
	}
	return v.String()
}

// TargetID returns the ID of the event's target element.
func (e *eventWasm) TargetID() string {
	v := e.Get("target").Get("id")
	if v.IsUndefined() || v.IsNull() {
		return ""
	}
	return v.String()
}

// TargetChecked returns the checked status of the event's target element.
func (e *eventWasm) TargetChecked() bool {
	v := e.Get("target").Get("checked")
	if v.IsUndefined() || v.IsNull() {
		return false
	}
	return v.Bool()
}

// Buttons returns the bitmask of currently pressed mouse/pointer buttons.
func (e *eventWasm) Buttons() int {
	v := e.Get("buttons")
	if v.IsUndefined() || v.IsNull() {
		return 0
	}
	return v.Int()
}

// ReleasePointerCapture releases pointer capture on the event target if active.
func (e *eventWasm) ReleasePointerCapture() {
	if target := e.Get("target"); !target.IsUndefined() && !target.IsNull() {
		if pid := e.Get("pointerId"); !pid.IsUndefined() && !pid.IsNull() {
			if fn := target.Get("releasePointerCapture"); !fn.IsUndefined() && !fn.IsNull() {
				target.Call("releasePointerCapture", pid)
			}
		}
	}
}

// Key returns the pressed key: the DOM KeyboardEvent.key verbatim, guarded
// like every other accessor — a non-keyboard event carries no key and reads
// as "" rather than panicking.
func (e *eventWasm) Key() Key {
	v := e.Get("key")
	if v.IsUndefined() || v.IsNull() {
		return ""
	}
	return Key(v.String())
}

