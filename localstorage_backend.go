//go:build !wasm

package dom

import . "webtyp.com/fmt"

// Fuera del navegador no hay localStorage. Las funciones existen para que un
// paquete compartido (una pantalla que también compila en el servidor para
// extraer su CSS) compile, y responden siempre "no disponible".

// LocalStorageAvailable is always false outside the browser.
func LocalStorageAvailable() bool { return false }

// LocalStorageGet always fails outside the browser.
func LocalStorageGet(key string) (string, error) { return "", Err(msgLocalStorageUnavailable) }

// LocalStorageSet always fails outside the browser.
func LocalStorageSet(key, value string) error { return Err(msgLocalStorageUnavailable) }

// LocalStorageDel always fails outside the browser.
func LocalStorageDel(key string) error { return Err(msgLocalStorageUnavailable) }

// LocalStorageClear always fails outside the browser.
func LocalStorageClear() error { return Err(msgLocalStorageUnavailable) }
