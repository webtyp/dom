//go:build wasm

package dom_test

import (
	"testing"

	. "webtyp.com/dom"
)

func TestLocalStorage_Basic(t *testing.T) {
	if !LocalStorageAvailable() {
		t.Skip("localStorage unavailable")
	}

	key := "test_key"
	val := "test_value"

	// Cleanup
	LocalStorageDel(key)

	// Roundtrip
	if err := LocalStorageSet(key, val); err != nil {
		t.Fatalf("LocalStorageSet failed: %v", err)
	}

	got, err := LocalStorageGet(key)
	if err != nil {
		t.Fatalf("LocalStorageGet failed: %v", err)
	}
	if got != val {
		t.Errorf("got %q, want %q", got, val)
	}

	// Delete
	if err := LocalStorageDel(key); err != nil {
		t.Fatalf("LocalStorageDel failed: %v", err)
	}

	got, err = LocalStorageGet(key)
	if err != nil {
		t.Fatalf("LocalStorageGet failed: %v", err)
	}
	if got != "" {
		t.Errorf("got %q after Del, want empty", got)
	}
}

func TestLocalStorage_MissingKey_ReturnsEmptyNilError(t *testing.T) {
	if !LocalStorageAvailable() {
		t.Skip("localStorage unavailable")
	}

	got, err := LocalStorageGet("non_existent_key")
	if err != nil {
		t.Fatalf("LocalStorageGet failed: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestLocalStorage_Set_Overwrites(t *testing.T) {
	if !LocalStorageAvailable() {
		t.Skip("localStorage unavailable")
	}

	key := "overwrite_key"
	LocalStorageSet(key, "v1")
	LocalStorageSet(key, "v2")

	got, _ := LocalStorageGet(key)
	if got != "v2" {
		t.Errorf("got %q, want %q", got, "v2")
	}
}

func TestLocalStorage_Clear_RemovesAll(t *testing.T) {
	if !LocalStorageAvailable() {
		t.Skip("localStorage unavailable")
	}

	LocalStorageSet("k1", "v1")
	LocalStorageSet("k2", "v2")

	if err := LocalStorageClear(); err != nil {
		t.Fatalf("LocalStorageClear failed: %v", err)
	}

	v1, _ := LocalStorageGet("k1")
	v2, _ := LocalStorageGet("k2")
	if v1 != "" || v2 != "" {
		t.Error("Clear did not remove all keys")
	}
}

func TestLocalStorage_SetEmptyValue(t *testing.T) {
	if !LocalStorageAvailable() {
		t.Skip("localStorage unavailable")
	}

	key := "empty_key"
	if err := LocalStorageSet(key, ""); err != nil {
		t.Fatalf("LocalStorageSet failed: %v", err)
	}

	got, _ := LocalStorageGet(key)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestLocalStorage_OversizedValue_ReturnsError(t *testing.T) {
	if !LocalStorageAvailable() {
		t.Skip("localStorage unavailable")
	}

	key := "large_key"
	// lsMaxValue = 64 * 1024
	largeVal := make([]byte, 65*1024)
	for i := range largeVal {
		largeVal[i] = 'a'
	}

	err := LocalStorageSet(key, string(largeVal))
	if err == nil {
		t.Error("Expected error for oversized value, got nil")
	}
}

func TestLocalStorage_QuotaGuard_ReturnsError(t *testing.T) {
	if !LocalStorageAvailable() {
		t.Skip("localStorage unavailable")
	}

	LocalStorageClear()

	// lsMaxBytes = 4 * 1024 * 1024
	// lsMaxValue = 64 * 1024
	// Fill it up
	val := make([]byte, 60*1024)
	for i := range val {
		val[i] = 'a'
	}
	sval := string(val)

	// 4MB / 60KB approx 68 entries
	for i := 0; i < 70; i++ {
		key := "fill_" + string(rune(i))
		err := LocalStorageSet(key, sval)
		if err != nil {
			// Success - we hit the budget limit
			return
		}
	}
	t.Error("Should have hit the budget limit")
}
