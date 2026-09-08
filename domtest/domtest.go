//go:build wasm

// Package domtest is the shared harness for tests that drive a live DOM.
//
// Every module that renders into a browser needs the same four things: a clean
// mount point, a way to find a node, a way to read it, and a way to act on it.
// Before this package each one wrote its own, they disagreed, and one of them
// (svg/tests) referenced another module's unexported test helpers and stopped
// compiling. This is that wiring, written once, where dom owns it.
//
// Build-tagged wasm and imported only by tests, so it contributes nothing to an
// application binary.
package domtest

import (
	"syscall/js"
	"testing"

	dom "webtyp.com/dom"
)

// Mount prepares a clean element with the given id in the page body and routes
// dom's log through t. Call it at the top of every test that renders.
func Mount(t *testing.T, id string) {
	doc := js.Global().Get("document")
	existing := doc.Call("getElementById", id)
	if !existing.IsNull() && !existing.IsUndefined() {
		existing.Set("innerHTML", "")
	} else {
		root := doc.Call("createElement", "div")
		root.Set("id", id)
		doc.Get("body").Call("appendChild", root)
	}
	dom.SetLog(func(v ...any) { t.Log(v...) })
}

// find resolves a CSS selector to a live node. Every exported function below
// starts here, so the document lookup and the null/undefined guard are written
// once instead of once per function.
func find(selector string) (js.Value, bool) {
	el := js.Global().Get("document").Call("querySelector", selector)
	if el.IsNull() || el.IsUndefined() {
		return js.Undefined(), false
	}
	return el, true
}

// Query returns the first element matching a CSS selector.
//
// If the selector matches no element or the element carries no ID attribute,
// Query returns (nil, false).
func Query(selector string) (dom.Reference, bool) {
	el, ok := find(selector)
	if !ok {
		return nil, false
	}
	id := el.Call("getAttribute", "id")
	if id.IsNull() || id.IsUndefined() || id.String() == "" {
		return nil, false
	}
	return dom.Get(id.String())
}

// Text returns an element's textContent, or "" when the selector matches nothing.
func Text(selector string) string {
	el, ok := find(selector)
	if !ok {
		return ""
	}
	return el.Get("textContent").String()
}

// Fire dispatches a bubbling event of the given type at the first element
// matching selector. No-op when nothing matches.
func Fire(selector, eventType string) {
	el, ok := find(selector)
	if !ok {
		return
	}
	fire(el, eventType)
}

// Fill sets an input's value and fires "input", the pair every form test needs.
func Fill(selector, value string) {
	el, ok := find(selector)
	if !ok {
		return
	}
	el.Set("value", value)
	fire(el, "input")
}

// fire dispatches a bubbling event at an already-resolved node, so Fill does not
// have to look the same selector up a second time.
func fire(el js.Value, eventType string) {
	opts := js.Global().Get("Object").New()
	opts.Set("bubbles", true)
	el.Call("dispatchEvent", js.Global().Get("Event").New(eventType, opts))
}
