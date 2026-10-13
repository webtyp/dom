//go:build !wasm

package dom_test

import (
	"testing"

	"webtyp.com/dom"
)

// Fuera del navegador no hay localStorage: las funciones existen (un paquete
// compartido compila en host) y responden "no disponible", nunca un valor.
func TestBackend_LocalStorageIsUnavailable(t *testing.T) {
	if dom.LocalStorageAvailable() {
		t.Error("LocalStorageAvailable() = true outside the browser")
	}
	if v, err := dom.LocalStorageGet("k"); v != "" || err == nil {
		t.Errorf("LocalStorageGet = %q, %v; want \"\" and an error", v, err)
	}
	if err := dom.LocalStorageSet("k", "v"); err == nil {
		t.Error("LocalStorageSet must fail outside the browser")
	}
	if err := dom.LocalStorageDel("k"); err == nil {
		t.Error("LocalStorageDel must fail outside the browser")
	}
	if err := dom.LocalStorageClear(); err == nil {
		t.Error("LocalStorageClear must fail outside the browser")
	}
}
