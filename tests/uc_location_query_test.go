//go:build wasm

package dom_test

import (
	"syscall/js"
	"testing"
	"webtyp.com/dom"
	"webtyp.com/fmt"
)

// Helper to set the URL
func setURL(url string) {
	js.Global().Get("history").Call("replaceState", js.Null(), "", url)
}

func TestWASM_GetQuery_Missing(t *testing.T) {
	setURL("/?payment=p1&status=PAID&empty=#home")
	v, ok := dom.GetQuery("missing")
	if ok || v != "" {
		t.Errorf("GetQuery(missing) = %q, %v, want \"\", false", v, ok)
	}
}

func TestWASM_GetQuery_PresentButEmpty(t *testing.T) {
	setURL("/?payment=p1&status=PAID&empty=#home")
	v, ok := dom.GetQuery("empty")
	if !ok || v != "" {
		t.Errorf("GetQuery(empty) = %q, %v, want \"\", true", v, ok)
	}
}

func TestWASM_GetQuery_DecodesValue(t *testing.T) {
	setURL("/?q=a%20b")
	v, ok := dom.GetQuery("q")
	if !ok || v != "a b" {
		t.Errorf("GetQuery(q) = %q, %v, want \"a b\", true", v, ok)
	}
}

func TestWASM_ClearQuery_KeepsPathAndHash(t *testing.T) {
	setURL("/somepath?payment=p1&status=PAID#home")
	initialLength := js.Global().Get("history").Get("length").Int()

	dom.ClearQuery()

	loc := js.Global().Get("location")
	pathname := loc.Get("pathname").String()
	search := loc.Get("search").String()
	hash := loc.Get("hash").String()

	if pathname != "/somepath" {
		t.Errorf("pathname = %q, want /somepath", pathname)
	}
	if search != "" {
		t.Errorf("search = %q, want \"\"", search)
	}
	if hash != "#home" {
		t.Errorf("hash = %q, want #home", hash)
	}

	finalLength := js.Global().Get("history").Get("length").Int()
	if initialLength != finalLength {
		t.Errorf("history length changed from %d to %d", initialLength, finalLength)
	}
}

func setupNavigatePostMock(t *testing.T) (js.Value, func()) {
	t.Helper()
	proto := js.Global().Get("HTMLFormElement").Get("prototype")
	originalSubmit := proto.Get("submit")

	mockData := js.Global().Get("Object").New()
	js.Global().Set("__mockNavigatePostData", mockData)

	mockSubmit := js.FuncOf(func(this js.Value, args []js.Value) any {
		mockData.Set("action", this.Get("action"))
		mockData.Set("method", this.Get("method"))

		inputs := js.Global().Get("Object").New()
		elements := this.Get("elements")
		length := elements.Get("length").Int()
		for i := 0; i < length; i++ {
			el := elements.Index(i)
			if el.Get("tagName").String() == "INPUT" && el.Get("type").String() == "hidden" {
				inputs.Set(el.Get("name").String(), el.Get("value"))
			}
		}
		mockData.Set("inputs", inputs)
		return nil
	})

	proto.Set("submit", mockSubmit)

	return mockData, func() {
		proto.Set("submit", originalSubmit)
		js.Global().Delete("__mockNavigatePostData")
		mockSubmit.Release()
	}
}

func TestWASM_NavigatePost_BuildsHiddenFormAndSubmits(t *testing.T) {
	mockData, teardown := setupNavigatePostMock(t)
	defer teardown()

	fields := []fmt.KeyValue{
		{Key: "token_ws", Value: "token123"},
		{Key: "other", Value: "val"},
	}

	dom.NavigatePost("https://gateway.example.com/pay", fields)

	if mockData.Get("action").String() != "https://gateway.example.com/pay" {
		t.Errorf("action = %q, want https://gateway.example.com/pay", mockData.Get("action").String())
	}
	if mockData.Get("method").String() != "post" {
		t.Errorf("method = %q, want post", mockData.Get("method").String())
	}

	inputs := mockData.Get("inputs")
	if inputs.Get("token_ws").String() != "token123" {
		t.Errorf("input token_ws = %q, want token123", inputs.Get("token_ws").String())
	}
	if inputs.Get("other").String() != "val" {
		t.Errorf("input other = %q, want val", inputs.Get("other").String())
	}
}

func TestWASM_NavigatePost_NoFields(t *testing.T) {
	mockData, teardown := setupNavigatePostMock(t)
	defer teardown()

	dom.NavigatePost("https://gateway.example.com/pay-empty", nil)

	if mockData.Get("action").String() != "https://gateway.example.com/pay-empty" {
		t.Errorf("action = %q, want https://gateway.example.com/pay-empty", mockData.Get("action").String())
	}
	if mockData.Get("method").String() != "post" {
		t.Errorf("method = %q, want post", mockData.Get("method").String())
	}

	inputs := mockData.Get("inputs")
	if js.Global().Get("Object").Call("keys", inputs).Get("length").Int() != 0 {
		t.Errorf("expected 0 inputs, got %v", js.Global().Get("Object").Call("keys", inputs).Get("length").Int())
	}
}
