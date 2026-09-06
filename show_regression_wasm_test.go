//go:build wasm

package dom

import (
	"testing"
)

// TestShowSecondToggleSharedContent guards the fix for the panic reported in
// webtyp/layout docs/BUG_DOM.md ("element ... is already a child of another
// element"), which killed the app on the SECOND open of a Show whose render
// callback closed over elements built outside it. Show no longer takes a
// callback: the subtree is built and attached once, so re-attachment is
// unrepresentable. This test pins the consumer scenario — shared content,
// repeated toggles — plus the semantics that come with build-once: node
// identity and live bindings across toggles.
func TestShowSecondToggleSharedContent(t *testing.T) {
	cond := NewBool(false)
	msg := NewString("Delete «laptop»?")
	body := NewElement("div").ID("shared-body").
		Child(NewElement("span").ID("shared-msg").BindText(msg))

	s := Show(cond, body)
	Render("app", s)
	container, ok := Get(s.GetID())
	if !ok {
		t.Fatal("Show container not mounted")
	}
	display := func() string {
		return container.(*elementWasm).val.Get("style").Get("display").String()
	}

	// Hidden at start: mounted, display:none.
	if _, ok := Get("shared-msg"); !ok {
		t.Fatal("content must be mounted while hidden")
	}
	if display() != "none" {
		t.Fatalf("expected display:none while hidden, got %q", display())
	}

	msgRef, _ := Get("shared-msg")
	msgNode := msgRef.(*elementWasm).val

	// Two full open/close cycles — the second open is what panicked before.
	for i := 0; i < 2; i++ {
		cond.Set(true)
		if display() == "none" {
			t.Fatalf("cycle %d: expected visible after Set(true)", i)
		}
		cond.Set(false)
		if display() != "none" {
			t.Fatalf("cycle %d: expected display:none after Set(false)", i)
		}
	}
	cond.Set(true)

	// Node identity survives toggles (no innerHTML re-render).
	if again, _ := Get("shared-msg"); !again.(*elementWasm).val.Equal(msgNode) {
		t.Error("node identity lost across toggles")
	}

	// Bindings keep patching while hidden; the subtree is current on re-show.
	msg.Set("Delete «desktop»?")
	cond.Set(false)
	if got := msgRef.(*elementWasm).val.Get("textContent").String(); got != "Delete «desktop»?" {
		t.Errorf("binding stale while hidden: %q", got)
	}
	cond.Set(true)
	if got := msgRef.(*elementWasm).val.Get("textContent").String(); got != "Delete «desktop»?" {
		t.Errorf("binding stale after re-show: %q", got)
	}
}
