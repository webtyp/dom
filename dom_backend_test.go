//go:build !wasm

package dom

import (
	"testing"
)

func TestBackendStubs(t *testing.T) {
	td := &tinyDOM{}
	d := newDom(td)

	if _, ok := d.(interface {
		Get(string) (Reference, bool)
	}).Get("test"); !ok {
		t.Error("get should return true (stub) on backend")
	}

	if err := d.Render("p", nil); err == nil {
		t.Error("Render should return error on backend")
	}

	if err := d.Append("p", nil); err == nil {
		t.Error("Append should return error on backend")
	}

	d.(interface{ update(string) }).update("")

	d.(interface{ unmount(Component) }).unmount(nil)
	d.OnHashChange(func(h string) {})
	if d.GetHash() != "" {
		t.Error("GetHash should return empty string on backend")
	}
	d.SetHash("test")
}

// TestOnScrollCaptureBackendIsANoop guarantees that a component registering a
// scroll listener in Init(ctx) still renders under SSR: the backend stub never
// invokes the handler.
func TestOnScrollCaptureBackendIsANoop(t *testing.T) {
	called := false
	OnScrollCapture(func(scrollTop float64) {
		called = true
	})
	if called {
		t.Error("OnScrollCapture backend stub should never invoke the handler")
	}
}

// TestOnUserActivityBackendIsANoop guarantees that a component registering a
// presence listener in Init(ctx) still renders under SSR: the backend stub
// never invokes the handler.
func TestOnUserActivityBackendIsANoop(t *testing.T) {
	called := false
	OnUserActivity(func() {
		called = true
	})
	if called {
		t.Error("OnUserActivity backend stub should never invoke the handler")
	}
}
