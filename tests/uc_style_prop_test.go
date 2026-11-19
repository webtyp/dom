//go:build wasm

package dom_test

import (
	"syscall/js"
	"testing"

	. "webtyp.com/dom"
	"webtyp.com/dom/domtest"
)

func TestStyleProp_AppliesCSSOMPropertyInBrowser(t *testing.T) {
	domtest.Mount(t, "style-prop-root")

	el := NewElement("div").ID("sp-box").
		StyleProp("--meter-fill", "65%").
		StyleProp("grid-row", "2").
		StyleProp("grid-row", "4")

	if err := Render("style-prop-root", el); err != nil {
		t.Fatalf("Render: %v", err)
	}

	node := js.Global().Get("document").Call("getElementById", "sp-box")
	if node.IsNull() || node.IsUndefined() {
		t.Fatal("rendered element #sp-box not found")
	}

	style := node.Get("style")
	if got := style.Call("getPropertyValue", "--meter-fill").String(); got != "65%" {
		t.Errorf("--meter-fill = %q, want %q", got, "65%")
	}
	if got := style.Call("getPropertyValue", "grid-row").String(); got != "4" {
		t.Errorf("grid-row = %q, want %q", got, "4")
	}
}

func TestBuildNode_CreatesSVGNamespace(t *testing.T) {
	domtest.Mount(t, "svg-ns-root")

	el := NewElement("svg").ID("ns-svg").
		Child(NewElement("use").ID("ns-use").Attr("href", "/sprite.abc.svg#plus"))

	if err := Render("svg-ns-root", el); err != nil {
		t.Fatalf("Render: %v", err)
	}

	doc := js.Global().Get("document")
	svgNode := doc.Call("getElementById", "ns-svg")
	useNode := doc.Call("getElementById", "ns-use")
	const svgNS = "http://www.w3.org/2000/svg"
	if got := svgNode.Get("namespaceURI").String(); got != svgNS {
		t.Errorf("svg namespaceURI = %q, want %q", got, svgNS)
	}
	if got := useNode.Get("namespaceURI").String(); got != svgNS {
		t.Errorf("use namespaceURI = %q, want %q", got, svgNS)
	}
}

func TestBindStyleProp_ReactiveCSSOMInBrowser(t *testing.T) {
	domtest.Mount(t, "bind-style-prop-root")

	fill := NewString("25%")
	el := NewElement("div").ID("bsp-box").
		BindStyleProp("--meter-fill", fill).
		BindStylePropFunc("width", func() string { return fill.Get() })

	if err := Render("bind-style-prop-root", el); err != nil {
		t.Fatalf("Render: %v", err)
	}

	style := js.Global().Get("document").Call("getElementById", "bsp-box").Get("style")
	if got := style.Call("getPropertyValue", "--meter-fill").String(); got != "25%" {
		t.Errorf("initial --meter-fill = %q, want 25%%", got)
	}
	if got := style.Call("getPropertyValue", "width").String(); got != "25%" {
		t.Errorf("initial width = %q, want 25%%", got)
	}

	fill.Set("80%")
	if got := style.Call("getPropertyValue", "--meter-fill").String(); got != "80%" {
		t.Errorf("updated --meter-fill = %q, want 80%%", got)
	}
	if got := style.Call("getPropertyValue", "width").String(); got != "80%" {
		t.Errorf("updated width = %q, want 80%%", got)
	}

	fill.Set("")
	if got := style.Call("getPropertyValue", "--meter-fill").String(); got != "" {
		t.Errorf("cleared --meter-fill = %q, want empty", got)
	}
}
