//go:build wasm

package dom

import "syscall/js"

// deferToMicrotask runs fn once, on its own JS microtask, decoupled from
// whatever call stack scheduled it. Used by reconcileChildren to keep a
// reactively-mounted component's Init() off the call stack of the async
// callback that triggered the mount — see docs/PLAN.md.
func deferToMicrotask(fn func()) {
	var jsFn js.Func
	jsFn = js.FuncOf(func(this js.Value, args []js.Value) any {
		jsFn.Release()
		go fn()
		return nil
	})
	js.Global().Call("queueMicrotask", jsFn)
}
