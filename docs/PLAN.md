# PLAN — `dom.DarkSchemeActive()`: read the color scheme the page actually resolved

> Master: `/home/cesar/.claude/plans/si-la-ui-se-drifting-treasure.md` (track A, gate C) · 2026-10-08

## Problem

`components/themetoggle` hardcodes `defaultTheme = light` and writes `data-theme` on `Init` even
when the user never chose one. With `webtyp/css` following the OS by default (and an app able to
declare `css.DefaultLight()`), the toggle must instead show the scheme in effect — declared by the
app or taken from the OS — and leave `<html>` untouched until the user clicks. Go code has no way
to read that today.

## API gate

1. **Prior art.** Web `matchMedia('(prefers-color-scheme: dark)')` and Flutter
   `MediaQuery.platformBrightnessOf` report the OS preference only. Android
   `Configuration.isNightModeActive` reports the mode in effect for the app. Here the effective one
   is what matters: `css.DefaultLight()` can override the OS, so we read what the browser resolved,
   not the preference.
2. **Name.** `dom.DarkSchemeActive()` — "is the dark scheme active?". A bool result, no parameter.
3. **Ledger.** Concepts +1 · no other way exists to read it from Go · ways to do it 0 net.
4. **Where.** `cssfeature.go`, which already runs the equivalent probe. DRY: one unexported probe
   returns the resolved color of `light-dark(rgb(1, 2, 3), rgb(4, 5, 6))`; `SupportsLightDark` is
   "resolved to either", `DarkSchemeActive` is "resolved to the second" (not cached: the toggle
   changes it).
5. **Deletes.** themetoggle's `defaultTheme` and its unconditional `SetDocumentAttr` in `Init`.

## Steps

1. Red test `tests/uc_colorscheme_test.go` (wasm): `color-scheme: light` on `<html>` → false;
   `color-scheme: dark` → true.
2. Extract `resolveLightDarkProbe()`; add `DarkSchemeActive()` (false when light-dark() is
   unsupported: those browsers are permanently light via the css fallback).
3. `gotest`; `gopush`.
