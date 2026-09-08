//go:build wasm

package domtest_test

import (
	"syscall/js"
	"testing"

	dom "webtyp.com/dom"
	"webtyp.com/dom/domtest"
)

func TestMount_ClearsExistingRoot(t *testing.T) {
	domtest.Mount(t, "test-root")
	doc := js.Global().Get("document")
	root := doc.Call("getElementById", "test-root")
	if root.IsNull() || root.IsUndefined() {
		t.Fatal("expected #test-root to exist")
	}
	root.Set("innerHTML", "<p>dirty</p>")

	domtest.Mount(t, "test-root")
	if got := root.Get("innerHTML").String(); got != "" {
		t.Errorf("Mount did not clear innerHTML, got %q", got)
	}
}

func TestQuery_MissAndHit(t *testing.T) {
	domtest.Mount(t, "test-root")

	ref, ok := domtest.Query("#non-existent")
	if ok || ref != nil {
		t.Errorf("Query on non-existent selector: want (nil, false), got (%v, %v)", ref, ok)
	}

	doc := js.Global().Get("document")
	root := doc.Call("getElementById", "test-root")
	el := doc.Call("createElement", "span")
	el.Set("id", "target-id")
	el.Set("textContent", "hello")
	root.Call("appendChild", el)

	ref, ok = domtest.Query("#target-id")
	if !ok || ref == nil {
		t.Fatalf("Query on existing element with ID: want (ref, true), got (%v, %v)", ref, ok)
	}
	if got := ref.GetAttr("id"); got != "target-id" {
		t.Errorf("ref.GetAttr(\"id\"): want \"target-id\", got %q", got)
	}
}

func TestText_EmptyOnMiss(t *testing.T) {
	domtest.Mount(t, "test-root")
	if got := domtest.Text(".nope"); got != "" {
		t.Errorf("Text(\".nope\") want \"\", got %q", got)
	}
}

func TestFireAndFill(t *testing.T) {
	domtest.Mount(t, "test-root")

	var lastVal string
	input := dom.NewElement("input").
		ID("test-input").
		On("input", func(e dom.Event) {
			lastVal = e.TargetValue()
		})

	if err := dom.Render("test-root", input); err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	domtest.Fill("#test-input", "Ana")

	if lastVal != "Ana" {
		t.Errorf("Fill(\"#test-input\", \"Ana\"): event handler saw %q, want \"Ana\"", lastVal)
	}
}
