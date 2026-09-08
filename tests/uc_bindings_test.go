//go:build wasm

package dom_test

import (
	"syscall/js"
	"testing"

	. "webtyp.com/dom"
)

// setupBindRoot prepares a clean #bind-root div in the page body.
func setupBindRoot() {
	doc := js.Global().Get("document")
	existing := doc.Call("getElementById", "bind-root")
	if !existing.IsNull() {
		existing.Set("innerHTML", "")
		return
	}
	root := doc.Call("createElement", "div")
	root.Set("id", "bind-root")
	doc.Get("body").Call("appendChild", root)
}

func queryText(selector string) string {
	el := js.Global().Get("document").Call("querySelector", selector)
	if el.IsNull() || el.IsUndefined() {
		return "<not found>"
	}
	return el.Get("textContent").String()
}

// BindTextComp — regression: wireBindings called Render() a second time,
// producing new auto-IDs that don't match the DOM, so BindText subscriptions
// silently targeted phantom elements and the DOM never updated on signal.Set().
type BindTextComp struct {
	Element
	label *SignalString
}

func (c *BindTextComp) Init(_ Ctx) { c.label = NewString("initial") }
func (c *BindTextComp) Render() *Element {
	return NewElement("div").ID(c.GetID()).
		Child(NewElement("span").Key("btc-span").BindText(c.label))
}

func TestBindText_UpdatesDOM(t *testing.T) {
	setupBindRoot()
	comp := &BindTextComp{}
	comp.SetID("btc-root")
	if err := Render("bind-root", comp); err != nil {
		t.Fatalf("Render: %v", err)
	}

	if got := queryText("[data-key='btc-span']"); got != "initial" {
		t.Fatalf("before Set: want 'initial', got %q", got)
	}

	comp.label.Set("updated")

	if got := queryText("[data-key='btc-span']"); got != "updated" {
		t.Errorf("after Set: want 'updated', got %q — BindText subscription targeting wrong ID (double-Render bug)", got)
	}
}

// CheckboxComp — regression: BindAttrBool("checked", sig) only set the content
// attribute. After a user toggles the checkbox, the live `.checked` IDL property
// no longer follows the attribute, so sig.Set(false) left the property stale at
// true. The next user click then toggled property→false and the "change" event
// reported the wrong state, so a CSS/JS toggle driven by the checkbox needed a
// second click to react ("works on the second time").
type CheckboxComp struct {
	Element
	open *SignalBool
}

func (c *CheckboxComp) Init(_ Ctx) { c.open = NewBool(true) }
func (c *CheckboxComp) Render() *Element {
	return NewElement("input").Attr("type", "checkbox").
		BindAttrBool("checked", c.open)
}

func TestBindAttrBool_SyncsCheckedProperty(t *testing.T) {
	setupBindRoot()
	comp := &CheckboxComp{}
	comp.SetID("cbx-root")
	if err := Render("bind-root", comp); err != nil {
		t.Fatalf("Render: %v", err)
	}

	cbx := js.Global().Get("document").Call("getElementById", "cbx-root")
	if !cbx.Get("checked").Bool() {
		t.Fatalf("initial: .checked property want true, got false")
	}

	comp.open.Set(false)

	if cbx.Get("checked").Bool() {
		t.Errorf("after Set(false): .checked property still true — attrbool binding did not sync the live property (would require a second user click to toggle)")
	}
}

// ChildBindComp / ParentWithChild — regression: child components embedded
// inside a parent's Render() never had wireBindings called for them, so their
// BindText/BindAttr bindings were never wired and signals had no DOM effect.
type ChildBindComp struct {
	Element
	value *SignalString
}

func (c *ChildBindComp) Init(_ Ctx) { c.value = NewString("child-initial") }
func (c *ChildBindComp) Render() *Element {
	return NewElement("p").ID(c.GetID()).
		Child(NewElement("span").Key("cbc-span").BindText(c.value))
}

type ParentWithChild struct {
	Element
	child ChildBindComp
}

func (p *ParentWithChild) Render() *Element {
	p.child.SetID("cbc-p")
	return NewElement("div").ID(p.GetID()).Child(&p.child)
}

func TestBindText_ChildComponent_UpdatesDOM(t *testing.T) {
	setupBindRoot()
	parent := &ParentWithChild{}
	parent.SetID("pwc-root")
	if err := Render("bind-root", parent); err != nil {
		t.Fatalf("Render: %v", err)
	}

	if got := queryText("[data-key='cbc-span']"); got != "child-initial" {
		t.Fatalf("before Set: want 'child-initial', got %q", got)
	}

	parent.child.value.Set("child-updated")

	if got := queryText("[data-key='cbc-span']"); got != "child-updated" {
		t.Errorf("after Set: want 'child-updated', got %q — child component bindings not wired (mountRecursive missing wireBindings)", got)
	}
}
