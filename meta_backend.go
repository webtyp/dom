//go:build !wasm

package dom

// GetMeta returns an empty string on the backend.
func GetMeta(_ string) string { return "" }
