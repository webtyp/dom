//go:build wasm

package dom_test

import (
	"testing"

	. "webtyp.com/dom"
)

type RefComp struct {
	Element
	row *Element
}

func (c *RefComp) Render() *Element {
	c.row = NewElement("span").Key("row").Text("a")
	return NewElement("div").ID(c.GetID()).Child(c.row)
}

func TestRef_ResolvesKeyedElement(t *testing.T) {
	setupBindRoot()
	comp := &RefComp{}
	comp.SetID("ref-comp-root")
	if err := Render("bind-root", comp); err != nil {
		t.Fatalf("Render: %v", err)
	}

	rowRef, ok := comp.row.Ref()
	if !ok {
		t.Fatalf("expected Ref() ok == true after Render, got false")
	}
	if rowRef == nil {
		t.Fatalf("expected non-nil Reference from Ref()")
	}

	rowRef.SetText("b")

	if got := queryText("#bind-root span"); got != "b" {
		t.Errorf("expected text content 'b' after SetText through Ref(), got %q", got)
	}
}

type TwoInstanceParent struct {
	Element
	child1 RefComp
	child2 RefComp
}

func (p *TwoInstanceParent) Render() *Element {
	p.child1.SetID("rc1")
	p.child2.SetID("rc2")
	return NewElement("div").ID(p.GetID()).Child(&p.child1, &p.child2)
}

func TestRef_TwoInstances_DoNotCollide(t *testing.T) {
	setupBindRoot()
	parent := &TwoInstanceParent{}
	parent.SetID("two-inst-root")
	if err := Render("bind-root", parent); err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	ref1, ok1 := parent.child1.row.Ref()
	ref2, ok2 := parent.child2.row.Ref()

	if !ok1 || !ok2 {
		t.Fatalf("expected both Ref() calls to succeed, got ok1=%v, ok2=%v", ok1, ok2)
	}

	// Compare the ids dom assigned, not the Reference values: Get allocates a
	// fresh wrapper per call, so `ref1 == ref2` would be false even if both
	// pointed at the SAME node — a vacuous assertion. The ids are what must
	// differ for the two instances not to collide.
	id1, id2 := ref1.GetAttr("id"), ref2.GetAttr("id")
	if id1 == "" || id2 == "" {
		t.Fatalf("both rows must carry a dom-assigned id, got %q and %q", id1, id2)
	}
	if id1 == id2 {
		t.Fatalf("two instances collided on id %q — the whole point of Ref()", id1)
	}

	ref1.SetText("updated-1")

	if got := queryText("#rc1 span"); got != "updated-1" {
		t.Errorf("expected child1 span text 'updated-1', got %q", got)
	}
	if got := queryText("#rc2 span"); got != "a" {
		t.Errorf("expected child2 span text to remain 'a', got %q", got)
	}
}
