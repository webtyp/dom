//go:build !wasm

package dom_test

import (
	"testing"
	"webtyp.com/dom"
	"webtyp.com/fmt"
)

func TestBackend_LocationHelpersAreNoops(t *testing.T) {
	// Tests: Q1-!wasm, C1-!wasm, N1-!wasm
	v, ok := dom.GetQuery("key")
	if v != "" || ok != false {
		t.Errorf("GetQuery() = %v, %v, want \"\", false", v, ok)
	}

	// Should not panic
	dom.ClearQuery()

	// Should not panic
	dom.NavigatePost("https://example.com", []fmt.KeyValue{{Key: "k", Value: "v"}})
}
