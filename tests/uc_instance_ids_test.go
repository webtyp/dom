//go:build wasm

package dom_test

import (
	"syscall/js"
	"testing"

	. "webtyp.com/dom"
)

type CellComponent struct {
	Element
	clicks *SignalBool
}

func (c *CellComponent) Init(_ Ctx) {
	c.clicks = NewBool(false)
}

func (c *CellComponent) Render() *Element {
	return NewElement("div").
		Child(
			NewElement("button").ID("cell").
				On("click", func(e Event) {
					c.clicks.Set(true)
				}),
		)
}

func TestSiblingComponents_DuplicateAuthorIds_Disambiguated(t *testing.T) {
	setupBindRoot()

	cellA := &CellComponent{}
	cellB := &CellComponent{}

	parent := NewElement("div").ID("sibling-parent").Child(cellA, cellB)
	if err := Render("bind-root", parent); err != nil {
		t.Fatalf("Render failed for sibling components: %v", err)
	}

	btnA, okA := GetByKey(cellA.GetID(), "cell")
	btnB, okB := GetByKey(cellB.GetID(), "cell")

	if !okA || btnA == nil {
		t.Fatalf("Button A not found via GetByKey under component %s", cellA.GetID())
	}
	if !okB || btnB == nil {
		t.Fatalf("Button B not found via GetByKey under component %s", cellB.GetID())
	}

	doc := js.Global().Get("document")
	jsBtnA := doc.Call("querySelector", "[id='"+cellA.GetID()+"'] [data-key='cell']")
	jsBtnB := doc.Call("querySelector", "[id='"+cellB.GetID()+"'] [data-key='cell']")

	// Trigger click on btnA
	jsBtnA.Call("click")

	if !cellA.clicks.Get() {
		t.Errorf("Cell A click handler was not invoked")
	}
	if cellB.clicks.Get() {
		t.Errorf("Cell B click handler was erroneously invoked by Cell A click")
	}

	// Trigger click on btnB
	jsBtnB.Call("click")

	if !cellB.clicks.Get() {
		t.Errorf("Cell B click handler was not invoked")
	}
}
