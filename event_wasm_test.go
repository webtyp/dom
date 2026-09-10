//go:build wasm

package dom

import (
	"syscall/js"
	"testing"
)

// TestOnKeyDownMovesFocus is the consumer-shaped proof the publication rule
// demands: it goes through the real stack a consumer uses — a real element
// tree, OnKeyDown on its container, a real KeyboardEvent dispatched via
// syscall/js — with no doubles. Step 4 is the exact thing calendarslider
// must do in components: a roving-focus implementation written against
// OnKeyDown moves document.activeElement. If this is awkward to write here,
// the API is awkward to use.
func TestOnKeyDownMovesFocus(t *testing.T) {
	ids := []string{"key-day-0", "key-day-1", "key-day-2"}
	var days []*Element
	for _, id := range ids {
		days = append(days, NewElement("button").ID(id).Text(id))
	}

	var got Key
	current := 0
	container := NewElement("div").ID("key-strip").
		OnKeyDown(func(e KeyEvent) {
			got = e.Key()
			switch e.Key() {
			case KeyArrowRight:
				if current+1 < len(days) {
					current++
				}
			case KeyArrowLeft:
				if current > 0 {
					current--
				}
			case KeyHome:
				current = 0
			case KeyEnd:
				current = len(days) - 1
			default:
				return
			}
			if ref, ok := days[current].Ref(); ok {
				ref.Focus()
			}
		})
	for _, d := range days {
		container.Child(d)
	}

	if err := Render("app", container); err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// focus the first day, like a Tab stop landing on the strip
	if ref, ok := days[0].Ref(); ok {
		ref.Focus()
	} else {
		t.Fatal("day 0 has no live reference after Render")
	}

	dispatchKey := func(targetID, key string) {
		ref, ok := Get(targetID)
		if !ok {
			t.Fatalf("no live reference for %s", targetID)
		}
		opts := js.Global().Get("Object").New()
		opts.Set("key", key)
		opts.Set("bubbles", true)
		evt := js.Global().Get("KeyboardEvent").New("keydown", opts)
		ref.(*elementWasm).val.Call("dispatchEvent", evt)
	}
	activeID := func() string {
		active := js.Global().Get("document").Get("activeElement")
		if active.IsNull() || active.IsUndefined() {
			return ""
		}
		return active.Get("id").String()
	}

	// ArrowRight on day 0: handler sees the typed key, focus moves to day 1
	dispatchKey(ids[0], "ArrowRight")
	if got != KeyArrowRight {
		t.Errorf("handler received %q, want %q", string(got), string(KeyArrowRight))
	}
	if id := activeID(); id != ids[1] {
		t.Errorf("after ArrowRight activeElement is %q, want %q", id, ids[1])
	}

	// Home on day 1: focus returns to day 0
	dispatchKey(ids[1], "Home")
	if got != KeyHome {
		t.Errorf("handler received %q, want %q", string(got), string(KeyHome))
	}
	if id := activeID(); id != ids[0] {
		t.Errorf("after Home activeElement is %q, want %q", id, ids[0])
	}
}
