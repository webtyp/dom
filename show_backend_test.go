//go:build !wasm

package dom

import (
	"strings"
	"testing"
)

func TestShowBackend(t *testing.T) {
	off := Show(NewBool(false), func() *Element { return NewElement("span").Text("x") })
	html := off.String()
	if strings.Contains(html, "<span") {
		t.Error("child must NOT be serialized while false (lazy mounting)")
	}

	on := Show(NewBool(true), func() *Element { return NewElement("span").Text("x") })
	if !strings.Contains(on.String(), "<span") {
		t.Error("visible Show must serialize child")
	}
}
