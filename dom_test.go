package dom

import (
	"testing"

	"webtyp.com/fmt"
)

func TestElementSetAttr(t *testing.T) {
	el := (&Element{tag: "div"}).Set(fmt.KeyValue{Key: "class", Value: "my-class"}, fmt.KeyValue{Key: "data-test", Value: "val"})
	html := elementToHTML(el)
	expected := "<div class='my-class' data-test='val'></div>"
	if html != expected {
		t.Errorf("Expected %s, got %s", expected, html)
	}
}

func TestElementAttrEscaping(t *testing.T) {
	el := (&Element{tag: "img"}).Set(fmt.KeyValue{Key: "src", Value: "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg'%3E%3C/svg%3E"})
	html := elementToHTML(el)
	expected := "<img src='data:image/svg+xml,%3Csvg xmlns=&#39;http://www.w3.org/2000/svg&#39;%3E%3C/svg%3E'></img>"
	if html != expected {
		t.Errorf("Expected %s, got %s", expected, html)
	}
}

type mockCSSProvider struct{}

func (m *mockCSSProvider) RenderCSS() any {
	return nil
}
