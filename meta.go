//go:build wasm

package dom

// GetMeta reads the content attribute of <meta name="..."> in the document.
// Returns "" if the element or content attribute is absent.
func GetMeta(name string) string {
	if name == "" || instance == nil {
		return ""
	}
	dw, ok := instance.(*domWasm)
	if !ok || !dw.document.Truthy() {
		return ""
	}
	meta := dw.document.Call("querySelector", `meta[name="`+name+`"]`)
	if meta.IsNull() || meta.IsUndefined() {
		return ""
	}
	v := meta.Call("getAttribute", "content")
	if v.IsNull() || v.IsUndefined() {
		return ""
	}
	return v.String()
}
