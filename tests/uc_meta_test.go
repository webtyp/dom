//go:build wasm

package dom_test

import (
	"syscall/js"
	"testing"

	. "webtyp.com/dom"
)

func TestGetMeta_ReadsContentAndReturnsEmptyWhenAbsent(t *testing.T) {
	if got := GetMeta("non-existent-meta"); got != "" {
		t.Fatalf("absent meta = %q, want empty", got)
	}

	doc := js.Global().Get("document")
	meta := doc.Call("createElement", "meta")
	meta.Call("setAttribute", "name", "webtyp-test-meta")
	meta.Call("setAttribute", "content", "/sprite.test123.svg")
	doc.Get("head").Call("appendChild", meta)
	t.Cleanup(func() { meta.Call("remove") })

	if got := GetMeta("webtyp-test-meta"); got != "/sprite.test123.svg" {
		t.Fatalf("GetMeta = %q, want %q", got, "/sprite.test123.svg")
	}
}
