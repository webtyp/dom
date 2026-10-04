//go:build wasm

package dom

import (
	"testing"
)

func TestShowSecondToggleSharedContent(t *testing.T) {
	cond := NewBool(false)
	msg := NewString("Delete laptop?")

	s := Show(cond, func() *Element {
		return NewElement("div").ID("shared-body").
			Child(NewElement("span").ID("shared-msg").BindText(msg))
	})
	Render("app", s)

	// Hidden at start: unmounted.
	if _, ok := Get("shared-msg"); ok {
		t.Fatal("content must NOT be mounted while false")
	}

	// Two full open/close cycles — must not panic and must mount/unmount cleanly.
	for i := 0; i < 2; i++ {
		cond.Set(true)
		if _, ok := Get("shared-msg"); !ok {
			t.Fatalf("cycle %d: expected mounted after Set(true)", i)
		}
		cond.Set(false)
		if _, ok := Get("shared-msg"); ok {
			t.Fatalf("cycle %d: expected unmounted after Set(false)", i)
		}
	}
	cond.Set(true)
	msgRef, ok := Get("shared-msg")
	if !ok {
		t.Fatal("expected mounted on final show")
	}

	msg.Set("Delete desktop?")
	if got := msgRef.(*elementWasm).val.Get("textContent").String(); got != "Delete desktop?" {
		t.Errorf("binding stale after update: %q", got)
	}
}
