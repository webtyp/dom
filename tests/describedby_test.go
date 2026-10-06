package dom_test

import (
	"testing"

	"webtyp.com/dom"
	"webtyp.com/fmt"
)

// DescribedBy ties a control to the elements that describe it (help text,
// error) through the IDs dom mints — never a hand-composed id.
func TestDescribedBy(t *testing.T) {
	help := dom.NewElement("small").Text("help")
	errSpan := dom.NewElement("span")
	input := dom.NewElement("input").DescribedBy(help, nil, errSpan)

	want := `aria-describedby='` + help.GetID() + " " + errSpan.GetID() + `'`
	if help.GetID() == "" || errSpan.GetID() == "" {
		t.Fatal("DescribedBy must mint an ID for each described element")
	}
	if got := input.String(); !fmt.Contains(got, want) {
		t.Errorf("got %s, want it to contain %s", got, want)
	}
}

func TestDescribedByNothing(t *testing.T) {
	input := dom.NewElement("input").DescribedBy(nil)
	if got := input.String(); fmt.Contains(got, "aria-describedby") {
		t.Errorf("no element given: attribute must not be set, got %s", got)
	}
}
