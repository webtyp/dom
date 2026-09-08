//go:build !wasm

package dom

import (
	"strings"
	"testing"
)

type dummyChildIdComp struct {
	id string
}

func (c *dummyChildIdComp) GetID() string         { return c.id }
func (c *dummyChildIdComp) SetID(id string)       { c.id = id }
func (c *dummyChildIdComp) String() string        { return elementToHTML(c.Render()) }
func (c *dummyChildIdComp) Children() []Component { return nil }
func (c *dummyChildIdComp) Render() *Element {
	return NewElement("div").ID("author-root").
		Child(NewElement("span").ID("author-child").Text("child text"))
}

func TestComponentChildId_IsDomGenerated(t *testing.T) {
	comp1 := &dummyChildIdComp{id: "comp-1"}
	comp2 := &dummyChildIdComp{id: "comp-2"}

	parent := NewElement("main").Child(comp1, comp2)
	html := elementToHTML(parent)

	if strings.Contains(html, "id='author-child'") || strings.Contains(html, "id=\"author-child\"") {
		t.Errorf("HTML should not contain author child ID 'id=author-child', got: %s", html)
	}
	if strings.Contains(html, "id='author-root'") || strings.Contains(html, "id=\"author-root\"") {
		t.Errorf("HTML should not contain author root ID 'id=author-root', got: %s", html)
	}

	if !strings.Contains(html, "id='comp-1'") || !strings.Contains(html, "id='comp-2'") {
		t.Errorf("HTML should contain component instance IDs 'comp-1' and 'comp-2', got: %s", html)
	}
}

func TestRootId_StillHonored(t *testing.T) {
	el := NewElement("div").ID("app")
	html := elementToHTML(el)

	if !strings.Contains(html, "id='app'") {
		t.Errorf("Top-level root element ID 'app' should be preserved, got: %s", html)
	}
}

func TestSameInstanceTwice_StillPanics(t *testing.T) {
	comp := &dummyChildIdComp{id: "shared-instance"}
	parent := NewElement("div").Child(comp, comp)

	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("Expected panic when rendering the same component instance twice in one render pass")
		}
	}()

	_ = elementToHTML(parent)
}

func TestGetByKey_Backend(t *testing.T) {
	ref, ok := GetByKey("owner", "key")
	if ref == nil {
		t.Errorf("GetByKey should return non-nil reference")
	}
	_ = ok
}
