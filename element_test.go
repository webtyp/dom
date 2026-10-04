package dom

import (
	"fmt"
	"strings"
	"testing"
)

func TestElement_ImplementsStringer(t *testing.T) {
	var _ fmt.Stringer = &Element{} // compile-time check
}

func TestElement_String_Basic(t *testing.T) {
	el := &Element{tag: "div"}
	el.Class("root").Text("hello")
	got := el.String()
	if !strings.Contains(got, "class='root'") {
		t.Error("expected class")
	}
	if !strings.Contains(got, "hello") {
		t.Error("expected text")
	}
}

func TestElement_PointerAndMouseEvents(t *testing.T) {
	called := 0
	el := &Element{tag: "div"}
	el.OnMouseDown(func(e Event) { called++ }).
		OnMouseUp(func(e Event) { called++ }).
		OnPointerDown(func(e Event) {
			called++
			e.ReleasePointerCapture()
			_ = e.Buttons()
		}).
		OnPointerUp(func(e Event) { called++ }).
		OnPointerEnter(func(e Event) { called++ }).
		OnPointerLeave(func(e Event) { called++ }).
		OnPointerMove(func(e Event) { called++ }).
		OnTouchStart(func(e Event) { called++ }).
		OnTouchMove(func(e Event) { called++ }).
		OnTouchEnd(func(e Event) { called++ })

	if len(el.events) != 10 {
		t.Errorf("expected 10 events registered, got %d", len(el.events))
	}
}

