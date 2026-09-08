//go:build wasm

package dom_test

import (
	"testing"

	. "webtyp.com/dom"
	"webtyp.com/dom/domtest"
)

// fakeState stands in for widget.State in the consumer-shaped test: dom must
// not import the widget vocabulary, so a StateAttr is anything with Key()/Value().
type fakeState struct{ key, value string }

func (s fakeState) Key() string   { return s.key }
func (s fakeState) Value() string { return s.value }

// StateComp — consumer-shaped proof for BindState: a state bound to a signal
// must render as data-x="true" and vanish when the signal flips. The value is
// asserted, never merely the attribute's presence: a presence test passes
// against the original defect (BindAttrBool wrote data-x="" and "the attribute
// is there" was true).
type StateComp struct {
	Element
	sel *SignalBool
}

func (c *StateComp) Init(_ Ctx) { c.sel = NewBool(true) }
func (c *StateComp) Render() *Element {
	return NewElement("div").ID(c.GetID()).
		Child(NewElement("span").ID("stc-row").
			BindState(fakeState{"data-selected", "true"}, c.sel))
}

func TestBindState_WritesValueAndRemoves(t *testing.T) {
	domtest.Mount(t, "bind-root")
	comp := &StateComp{}
	comp.SetID("stc-root")
	if err := Render("bind-root", comp); err != nil {
		t.Fatalf("Render: %v", err)
	}

	row, ok := Get("stc-row")
	if !ok {
		t.Fatal("stc-row not mounted")
	}
	if got := row.GetAttr("data-selected"); got != "true" {
		t.Fatalf("initial: data-selected want \"true\", got %q", got)
	}

	comp.sel.Set(false)

	if got := row.GetAttr("data-selected"); got != "<null>" {
		t.Errorf("after Set(false): data-selected want absent, got %q — BindState did not remove the attribute", got)
	}

	comp.sel.Set(true)

	if got := row.GetAttr("data-selected"); got != "true" {
		t.Errorf("after Set(true): data-selected want \"true\", got %q", got)
	}
}

// StateFuncComp — BindStateFunc: the computed form writes the same value; the
// state itself decides the value, so markup and CSS cannot disagree.
type StateFuncComp struct {
	Element
	a, b *SignalBool
}

func (c *StateFuncComp) Init(_ Ctx) {
	c.a = NewBool(false)
	c.b = NewBool(true)
}
func (c *StateFuncComp) Render() *Element {
	return NewElement("div").ID(c.GetID()).
		Child(NewElement("span").ID("sf-row").
			BindStateFunc(fakeState{"data-current", "true"},
				func() bool { return c.a.Get() && c.b.Get() }))
}

func TestBindStateFunc_ComputedWritesValue(t *testing.T) {
	domtest.Mount(t, "bind-root")
	comp := &StateFuncComp{}
	comp.SetID("sf-root")
	if err := Render("bind-root", comp); err != nil {
		t.Fatalf("Render: %v", err)
	}

	row, ok := Get("sf-row")
	if !ok {
		t.Fatal("sf-row not mounted")
	}
	if got := row.GetAttr("data-current"); got != "<null>" {
		t.Fatalf("initial: data-current present (a=false), want absent, got %q", got)
	}

	comp.a.Set(true)

	if got := row.GetAttr("data-current"); got != "true" {
		t.Errorf("after Set(true): data-current want \"true\", got %q", got)
	}
}
