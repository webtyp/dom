//go:build wasm

package dom_test

import (
	"testing"

	. "webtyp.com/dom"
)

func TestDocumentAttr_Basic(t *testing.T) {
	attr := "data-test-attr"
	val := "test-value"

	// Cleanup
	SetDocumentAttr(attr, "")

	// Roundtrip
	SetDocumentAttr(attr, val)
	got := GetDocumentAttr(attr)
	if got != val {
		t.Errorf("got %q, want %q", got, val)
	}

	// Remove
	SetDocumentAttr(attr, "")
	got = GetDocumentAttr(attr)
	if got != "" {
		t.Errorf("got %q after removal, want empty", got)
	}
}

func TestGetDocumentAttr_NoAttribute_ReturnsEmpty(t *testing.T) {
	got := GetDocumentAttr("non-existent-attr")
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestSetDocumentAttr_PassesThrough_AnyString(t *testing.T) {
	attr := "data-xyz"
	val := "literal-string"
	SetDocumentAttr(attr, val)
	got := GetDocumentAttr(attr)
	if got != val {
		t.Errorf("got %q, want %q", got, val)
	}
}
