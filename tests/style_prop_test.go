package dom_test

import (
	"testing"

	. "webtyp.com/dom"
)

func TestStyleProp_NotEmittedAsHTMLAttr(t *testing.T) {
	el := NewElement("div").StyleProp("--meter-fill", "40%").Text("ok")
	got := el.String()
	if containsSub(got, "style=") {
		t.Fatalf("StyleProp must not emit inline style= HTML attribute, got: %s", got)
	}
	if got != "<div>ok</div>" {
		t.Fatalf("want <div>ok</div>, got %q", got)
	}
}

func TestStyleProp_LastValueWins(t *testing.T) {
	el := NewElement("div").
		StyleProp("grid-row", "1").
		StyleProp("grid-column", "2").
		StyleProp("grid-row", "3")
	if got := el.String(); got != "<div></div>" {
		t.Fatalf("want <div></div>, got %q", got)
	}
}

func TestStyleProp_IgnoresEmptyProperty(t *testing.T) {
	el := NewElement("span").StyleProp("", "10px").Text("x")
	if got := el.String(); got != "<span>x</span>" {
		t.Fatalf("want <span>x</span>, got %q", got)
	}
}

func TestStyleAttr_PanicsOnInlineStyle(t *testing.T) {
	cases := []struct {
		name string
		fn   func()
	}{
		{"Attr lowercase", func() { NewElement("div").Attr("style", "color:red") }},
		{"Attr uppercase", func() { NewElement("div").Attr("STYLE", "color:red") }},
		{"BindAttr", func() { NewElement("div").BindAttr("style", NewString("color:red")) }},
		{"BindAttrFunc", func() { NewElement("div").BindAttrFunc("style", func() string { return "color:red" }) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Fatalf("expected panic for %s", tc.name)
				}
			}()
			tc.fn()
		})
	}
}

func TestBindStyleProp_NotEmittedAsHTMLAttr(t *testing.T) {
	sig := NewString("40%")
	el := NewElement("div").
		BindStyleProp("--meter-fill", sig).
		BindStylePropFunc("width", func() string { return sig.Get() })
	if got := el.String(); containsSub(got, "style=") {
		t.Fatalf("BindStyleProp must not emit inline style= HTML attribute, got: %s", got)
	}
}
