//go:build wasm

package dom_test

import (
	"testing"
	"time"

	. "webtyp.com/dom"
	"webtyp.com/dom/domtest"
)

type UARoot struct {
	Element
}

func (c *UARoot) Render() *Element {
	return NewElement("div").ID("ua-root")
}

func TestOnUserActivity(t *testing.T) {
	domtest.Mount(t, "app")
	root := &UARoot{}
	if err := Render("app", root); err != nil {
		t.Fatalf("Render: %v", err)
	}

	count := 0
	OnUserActivity(func() {
		count++
	})

	// 1. Fire pointermove: should increment counter to 1
	domtest.Fire("#ua-root", "pointermove")
	if count != 1 {
		t.Fatalf("case 1 (first event): want count 1, got %d", count)
	}

	// 2. Fire pointermove again immediately: should be throttled (count stays 1)
	domtest.Fire("#ua-root", "pointermove")
	if count != 1 {
		t.Fatalf("case 2 (throttled): want count 1, got %d", count)
	}

	// 3. Sleep 1100ms for window to reopen, fire keydown: counter increments to 2
	time.Sleep(1100 * time.Millisecond)
	domtest.Fire("#ua-root", "keydown")
	if count != 2 {
		t.Fatalf("case 3 (reopened window, keydown): want count 2, got %d", count)
	}
}
