//go:build wasm

package dom

import . "webtyp.com/fmt"

const lsMaxBytes = 4 * 1024 * 1024 // presupuesto total (bytes UTF-16 estimados; cuota típica 5MB)
const lsMaxValue = 64 * 1024       // límite por valor individual — O(1) sin llamada JS

// LocalStorageAvailable reports whether localStorage is accessible in the current browser context.
// Returns false when blocked by iframe sandbox, privacy settings, or private mode.
// Used internally by all LocalStorage* functions and available as a public check.
func LocalStorageAvailable() bool {
	return instance.(*domWasm).localStorage.Truthy()
}

// LocalStorageGet retrieves a value from window.localStorage.
// Returns ("", nil)   — key absent, storage is functional.
// Returns ("", error) — storage unavailable.
func LocalStorageGet(key string) (string, error) {
	if !LocalStorageAvailable() {
		return "", Err("localStorage unavailable")
	}
	v := instance.(*domWasm).localStorage.Call("getItem", key)
	if v.IsNull() || v.IsUndefined() {
		return "", nil
	}
	return v.String(), nil
}

// LocalStorageSet writes a key-value pair.
// Returns error if: storage unavailable, value > lsMaxValue, or budget exceeded.
func LocalStorageSet(key, value string) error {
	if !LocalStorageAvailable() {
		return Err("localStorage unavailable")
	}
	if len(value) > lsMaxValue {
		return Errf("localStorage value too large for key %s", key)
	}
	d := instance.(*domWasm)
	newSize := lsEntrySize(key, value)
	oldSize := 0
	if existing := d.localStorage.Call("getItem", key); !existing.IsNull() && !existing.IsUndefined() {
		oldSize = lsEntrySize(key, existing.String())
	}
	if d.lsUsedBytes+newSize-oldSize > lsMaxBytes {
		return Errf("localStorage budget exceeded for key %s", key)
	}
	d.localStorage.Call("setItem", key, value)
	d.lsUsedBytes += newSize - oldSize
	return nil
}

// LocalStorageDel removes a key. Returns error if storage unavailable.
func LocalStorageDel(key string) error {
	if !LocalStorageAvailable() {
		return Err("localStorage unavailable")
	}
	d := instance.(*domWasm)
	if existing := d.localStorage.Call("getItem", key); !existing.IsNull() && !existing.IsUndefined() {
		d.lsUsedBytes -= lsEntrySize(key, existing.String())
	}
	d.localStorage.Call("removeItem", key)
	return nil
}

// LocalStorageClear removes all keys. Returns error if storage unavailable.
func LocalStorageClear() error {
	if !LocalStorageAvailable() {
		return Err("localStorage unavailable")
	}
	d := instance.(*domWasm)
	d.lsUsedBytes = 0
	d.localStorage.Call("clear")
	return nil
}
