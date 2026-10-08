//go:build wasm

package dom_test

import (
	"testing"

	. "webtyp.com/dom"
)

// TestDarkSchemeActive_FollowsResolvedScheme drives the scheme the way a
// stylesheet would (color-scheme on <html>) and checks the effective value is
// what comes back — not the OS preference, which a page can override.
func TestDarkSchemeActive_FollowsResolvedScheme(t *testing.T) {
	if !SupportsLightDark() {
		t.Skip("browser cannot resolve light-dark(); the page is permanently light")
	}
	defer SetDocumentAttr("style", "")

	SetDocumentAttr("style", "color-scheme: light")
	if DarkSchemeActive() {
		t.Error("color-scheme: light must read as not dark")
	}

	SetDocumentAttr("style", "color-scheme: dark")
	if !DarkSchemeActive() {
		t.Error("color-scheme: dark must read as dark")
	}
}
