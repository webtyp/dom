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

// Query returns the first element matching a CSS selector.
//
// If the selector matches no element or the element carries no ID attribute,
// Query returns (nil, false).
func Query(selector string) (dom.Reference, bool) {
	doc := js.Global().Get("document")
	el := doc.Call("querySelector", selector)
	if el.IsNull() || el.IsUndefined() {
		return nil, false
	}
	idAttr := el.Call("getAttribute", "id")
	if idAttr.IsNull() || idAttr.IsUndefined() || idAttr.String() == "" {
		return nil, false
	}
	return dom.Get(idAttr.String())
}

// Text returns an element's textContent, or "" when the selector matches nothing.
func Text(selector string) string {
	doc := js.Global().Get("document")
	el := doc.Call("querySelector", selector)
	if el.IsNull() || el.IsUndefined() {
		return ""
	}
	return el.Get("textContent").String()
}

// Fire dispatches a bubbling event of the given type at the first element
// matching selector. No-op when nothing matches.
func Fire(selector, eventType string) {
	doc := js.Global().Get("document")
	el := doc.Call("querySelector", selector)
	if el.IsNull() || el.IsUndefined() {
		return
	}
	opts := js.Global().Get("Object").New()
	opts.Set("bubbles", true)
	evt := js.Global().Get("Event").New(eventType, opts)
	el.Call("dispatchEvent", evt)
}

// Fill sets an input's value and fires "input", the pair every form test needs.
func Fill(selector, value string) {
	doc := js.Global().Get("document")
	el := doc.Call("querySelector", selector)
	if el.IsNull() || el.IsUndefined() {
		return
	}
	el.Set("value", value)
	Fire(selector, "input")
}
