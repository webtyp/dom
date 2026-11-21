package dom_test

import (
	"strings"
	"testing"

	. "webtyp.com/dom"
)

// Show's wrapper must not take part in layout: an empty wrapper inside a flex/grid container with a gap would
// add one gap per hidden block (a stray strip under a header), and a filled one would sit between the container
// and the child, so Grow() and friends on the child stop applying. <slot> outside a shadow tree is display:
// contents in every user agent: no box, no gap, the child lays out as the container's own item.
func TestShow_WrapperTakesNoBox(t *testing.T) {
	hidden := Show(NewBool(false), func() *Element { return NewElement("span").Text("x") }).String()
	if !strings.HasPrefix(hidden, "<slot") {
		t.Fatalf("hidden Show: want a <slot> wrapper, got %q", hidden)
	}
	shown := Show(NewBool(true), func() *Element { return NewElement("span").Text("x") }).String()
	if !strings.HasPrefix(shown, "<slot") || !strings.Contains(shown, "<span") {
		t.Fatalf("shown Show: want <slot> around the child, got %q", shown)
	}
}
