//go:build wasm

package dom

import "syscall/js"

// supportsLightDarkCache memoizes SupportsLightDark: the probe is a real DOM
// read, not a syntax query, so it is worth paying once per page load and
// never again — see the function's own doc comment for why it cannot be a
// cheaper CSS.supports()/@supports check instead.
var supportsLightDarkCache *bool

// SupportsLightDark reports whether the browser can actually RESOLVE the CSS
// light-dark() color function — not merely parse it.
//
// This is deliberately a BEHAVIORAL probe: it applies
// "light-dark(rgb(1,2,3), rgb(4,5,6))" to a real, briefly-attached element
// and reads the resolved backgroundColor back, rather than asking
// CSS.supports('color', 'light-dark(red, blue)') (the JS equivalent of
// @supports). Both of those report whether the browser recognizes the
// SYNTAX; Safari 17.4 is a confirmed case of a browser that recognizes
// light-dark() but does not correctly resolve it — a syntax-only check
// would report "supported" on exactly the browser this function exists to
// catch. See webtyp.com/css's Token.EnhancedVar doc comment for
// the wider legacy-fallback investigation this came out of; a browser this
// reports false for is the same population that relies on that fallback,
// which is why this exists — a component whose only effect is toggling
// light-dark()-driven color (webtyp.com/components/themetoggle)
// has nothing to do on such a browser and should not render a control that
// looks broken when pressed.
//
// color-mix() needs no equivalent probe: every engine shipped it before
// light-dark() (Safari 16.2 vs 17.5; Chrome 111 vs 123; Firefox 113 vs
// 120), so a browser this reports true for already has it.
func SupportsLightDark() bool {
	if supportsLightDarkCache != nil {
		return *supportsLightDarkCache
	}

	d := instance.(*domWasm)
	probe := d.document.Call("createElement", "div")
	probe.Get("style").Set("cssText", "position:absolute;left:-9999px;background-color:light-dark(rgb(1, 2, 3), rgb(4, 5, 6));")

	body := d.document.Get("body")
	body.Call("appendChild", probe)
	computed := js.Global().Call("getComputedStyle", probe).Get("backgroundColor").String()
	body.Call("removeChild", probe)

	supported := computed == "rgb(1, 2, 3)" || computed == "rgb(4, 5, 6)"
	supportsLightDarkCache = &supported
	return supported
}
