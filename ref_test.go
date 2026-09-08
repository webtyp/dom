//go:build !wasm

package dom

import (
	"strings"
	"testing"
)

func TestRef_BeforeRender_False(t *testing.T) {
	el := NewElement("span").Key("k")
	ref, ok := el.Ref()
	if ok {
		t.Errorf("expected ok == false before render, got true with ref %v", ref)
	}
	if ref != nil {
		t.Errorf("expected ref == nil before render, got %v", ref)
	}
}

func TestSSR_KeyEmitsNoID(t *testing.T) {
	el := NewElement("div").Key("k")

	html1 := el.String()
	if strings.Contains(html1, "id=") {
		t.Errorf("expected SSR html to contain no id attribute, got %q", html1)
	}

	html2 := el.String()
	if html1 != html2 {
		t.Errorf("expected serializing same element twice to be byte-identical, got %q vs %q", html1, html2)
	}
}
