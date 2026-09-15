---
PLAN: "fix: reactive reconcile must not run a newly-mounted child's Init() nested inside the async callback that triggered the mount"
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 2311043536796432607
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# Plan — reactive mount deadlocks the whole wasm program when triggered from an async callback

## Context (read this before touching anything)

This repo is `webtyp.com/dom`, the framework's DOM/reconciliation layer. A downstream
application (`veltylabs/mjosefa-cms`, a real production app built on this ecosystem) was
crashing its entire client-side program — silently, with **zero** panic text reaching the
browser console — the instant a user navigated to certain screens. Root-caused with a
standalone Playwright script reading Chrome's real console/pageerror events directly (this
app's own MCP-based browser tooling never surfaces real `console.log`/panic text — only a
generic `[Issue] GenericIssue` feed). That surfaced the true Go runtime output:

```
fatal error: all goroutines are asleep - deadlock!

goroutine 1 [select (no cases)]:
main.main()
	web/client.go:30

goroutine 7 [chan receive]:
appointment_booking.(*reservationFormStore).List(...)
	lister.go:63
webtyp.com/view.(*core).Reload(...)
	presenter.go:48
webtyp.com/layout/crudview.(*CrudView).Reload(...)
	crudview.go:339
webtyp.com/layout/crudview.(*CrudView).Init(...)
	crudview.go:240
webtyp.com/dom.(*domWasm).initComponent(...)
	dom_frontend.go:244
webtyp.com/dom.(*domWasm).renderToHTML.func1(...) → writeElement → serializeElement → renderToHTML
webtyp.com/dom.(*domWasm).reconcileChildren(...)
	dom_frontend.go:974
webtyp.com/dom.(*domWasm).wireElementBindings.func8()
	dom_frontend.go:845
webtyp.com/dom.notify(...) → (*SignalNodes).Set(...)
	signal.go:21 / :149
mjosefa-cms/modules/appointment_booking.(*BookingView).rebuildPanel(...)
	bookingview.go:284
(*BookingView).reloadServices.func1/func3(...)
	bookingview.go:185,193
webtyp.com/mcp.(*mcpCaller).Call.func1 → (*Client).Call.func1 → webtyp.com/fetch.doRequest.func6
syscall/js.handleEvent()
```

## The actual mechanism

1. A consumer component's `reloadServices()` fires two async `caller.Call(...)`s (a real
   fetch-backed `router.Caller` — resolves on a *later* JS microtask, never inline).
2. When both resolve, the callback (running from inside `syscall/js.handleEvent()`, i.e.
   *nested inside an already-in-flight JS callback*, not from `main()`'s own top-level call
   stack) calls a method that does `someSignalNodes.Set([]*Element{ ...newly built subtree... })`.
3. `SignalNodes.Set` → `notify` → a subscriber wired by `wireElementBindings` →
   `reconcileChildren` (`dom_frontend.go:902`) sees a new node and, in its "create and insert"
   branch (`dom_frontend.go:974`, `html := d.renderToHTML(n, &comps, parentID)`), calls
   `renderToHTML` → `initComponent` (`dom_frontend.go:232`) **synchronously**, on the exact same
   call stack, for the newly-appeared child component.
4. That child's `Init(ctx)` is `crudview.CrudView.Init` (or any consumer following the same,
   widely-used and otherwise perfectly legitimate idiom — `webtyp.com/view`'s own
   `callerLister.list/save/update/delete`, and `appointment_booking`'s own
   `reservationFormStore.List`): `ch := make(chan error, 1); caller.Call(...); <-ch`. This
   pattern is **documented elsewhere in this ecosystem as safe** — see
   `veltylabs/mjosefa-cms/config/client.go`'s own comment on why blocking on `<-meDone` is fine
   for its "me" call: *"BuildClient se llama desde main, no desde un callback de JS... hacer lo
   mismo DENTRO de un callback de fetch SÍ es un deadlock."* `initComponent`, called from
   `reconcileChildren`, currently does exactly that: it runs a consumer's `Init()` **from
   inside a fetch callback**, silently violating the one precondition that idiom depends on.
5. Nothing can ever resolve step 4's channel: we are already nested inside one async callback
   with no way to return to the browser event loop until this new blocking call also resolves,
   and *that* resolution is itself just another opaque JS Promise callback waiting for its own
   turn — a turn that can only come after the current (already-executing) call stack unwinds.
   Go's own js/wasm scheduler correctly concludes there is no Go-visible pending work (a real
   `time.After`/`time.Sleep` timer WOULD be visible to it and would avoid this — a raw
   `fetch`/`setTimeout`-backed Promise is not) and every goroutine is asleep, so it calls
   `runtime.wasmExit(0)` — **silently**, no fatal-error text reaches `console`, because the js/
   wasm exit path in this app's own `wasm_exec_go.js` shim only logs a message for a *nonzero*
   exit code. The program is dead; any later-arriving fetch response that tries to resume it
   throws the generic, unhelpful `Go program has already exited`.

**This is not specific to `crudview`, `CallerLister`, or `reservationFormStore.` Any component
whose `Init()` performs a blocking round-trip — a normal, previously-safe pattern this
ecosystem already documents and uses in several places — will deadlock the instant it is
mounted reactively (via any `Signal.Set()`) from inside an already-async callback instead of
from the composition root's own synchronous startup.** The defect is in the one place that
decides when `Init()` runs relative to the caller's own call stack: `dom`'s reconcile.

## Documented `Init()` contract (must not regress)

`docs/BINDING_MODEL.md:101`: *"`Init(ctx)` — la preparación. Imperativa y corre exactamente una
vez, antes del [primer] `Render()`."* The fix below preserves both halves of this exactly:
`Init()` still runs precisely once per component, and still strictly before that same
component's first `Render()` — it only changes *which call stack* that pair runs on when the
mount is reactive, never for the initial composition-root render.

## The fix

`initComponent` itself must stay unaware of context — it is shared by the safe top-level path
(`Append`, the initial `dom.Render(parentID, component)`) and the reactive path
(`reconcileChildren`), and the top-level path's existing synchronous guarantee must not change
(several consumers, e.g. `config/client.go`'s `Focus()`-before-`ShowMain()` comment in the
mjosefa-cms app, rely on synchronous-after-mount ordering for the very first paint). Gate the
fix at `reconcileChildren`'s own "create and insert" branch instead:

1. Add an unexported field to `domWasm` (`dom_frontend.go`, near `currentComponentID`):
   ```go
   reconciling bool // true while inside reconcileChildren's own call stack — see docs/PLAN.md
   ```

2. In `reconcileChildren` (`dom_frontend.go:902`), set it for the duration of the function:
   ```go
   func (d *domWasm) reconcileChildren(parentID string, newNodes []*Element) {
   	d.reconciling = true
   	defer func() { d.reconciling = false }()
   	// ... existing body, unchanged ...
   }
   ```

3. In the "create and insert" branch (`dom_frontend.go:974`, currently:
   ```go
   } else {
   	// Create and insert
   	html := d.renderToHTML(n, &comps, parentID)
   	tempDiv := d.document.Call("createElement", "div")
   	tempDiv.Set("innerHTML", html)
   	newNode := tempDiv.Get("firstElementChild")
   	if i < existingLen {
   		parentVal.Call("insertBefore", newNode, existingNodes.Call("item", i))
   	} else {
   		parentVal.Call("appendChild", newNode)
   	}
   	d.wireElementBindings(n, parentID)
   }
   ```
   ), when `d.reconciling` is true (it always is here, but read it *before* clearing it — see
   below), defer the **entire block** — not just `initComponent`'s call inside `renderToHTML` —
   to a microtask, so `Init()` → `Render()` → HTML string → DOM insertion → binding wiring all
   still happen in that exact order, in one shot, just decoupled from the triggering callback's
   own call stack:
   ```go
   } else {
   	// Create and insert. Deferred to a microtask: Init() (called inside
   	// renderToHTML → initComponent) may block on a channel a LATER async
   	// callback fills — safe only when it is NOT nested inside the async
   	// callback that triggered this reconcile. See docs/PLAN.md (D16-class
   	// deadlock) for the full mechanism and the reproduction this guards.
   	nCopy, parentIDCopy, iCopy := n, parentID, i
   	deferToMicrotask(func() {
   		var comps []Component
   		html := d.renderToHTML(nCopy, &comps, parentIDCopy)
   		tempDiv := d.document.Call("createElement", "div")
   		tempDiv.Set("innerHTML", html)
   		newNode := tempDiv.Get("firstElementChild")
   		existing := parentVal.Call("querySelectorAll", ":scope > *")
   		if iCopy < existing.Get("length").Int() {
   			parentVal.Call("insertBefore", newNode, existing.Call("item", iCopy))
   		} else {
   			parentVal.Call("appendChild", newNode)
   		}
   		d.wireElementBindings(nCopy, parentIDCopy)
   	})
   }
   ```
   Re-resolve `existingNodes`/`existingLen` fresh inside the deferred closure (as
   `existing`/`existing.Get("length").Int()` above) rather than capturing the outer loop's
   stale snapshot — by the time the microtask runs, sibling insertions from the same
   reconcile pass may have already changed the DOM. Adjust exactly to match whatever the
   surrounding function's real variable names are; the shape (defer the whole insert, recompute
   position fresh) is what matters, not the identifiers.

4. Add the one-shot microtask helper (new file `microtask_wasm.go`, `//go:build wasm`,
   `package dom`) — TinyGo-compatible: `js.Global()`/`js.FuncOf`/`.Call()` are already used
   throughout this exact file (`dom_frontend.go:1053` and others) and throughout this
   ecosystem's TinyGo-targeted wasm builds (e.g. `veltylabs/mjosefa-cms`'s own
   `config/client.go` `setupIdleLock` uses `js.Global().Call("setTimeout", ...)` directly in
   a real TinyGo `-opt=z` production build):
   ```go
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
   		fn()
   		return nil
   	})
   	js.Global().Call("queueMicrotask", jsFn)
   }
   ```
   `Append` (the other, top-level caller of `initComponent`) is untouched — it never sets
   `d.reconciling`, so nothing about the initial composition-root mount changes.

## Stage 1 — add the reproduction FIRST, verify it actually reproduces

Before writing the fix, add this test **verbatim** and confirm it demonstrates the bug against
the current, unfixed code. This is not optional: it is the proof the defect is real, not
invented, and it becomes the regression test the fix must turn green.

Create `tests/uc_reactive_mount_deadlock_test.go`:

```go
//go:build wasm

package dom_test

import (
	"syscall/js"
	"testing"

	. "webtyp.com/dom"
	"webtyp.com/dom/domtest"
)

// asyncCall fires fn on a LATER JS macrotask — never synchronously inline —
// the same shape a real fetch-based router.Caller callback takes. It is
// deliberately NOT a Go-runtime timer (time.After/time.Sleep): those ARE
// visible to the js/wasm scheduler's own deadlock detector, which lets the
// runtime "sleep" instead of declaring a real deadlock — silently masking
// the exact hazard this test exists to catch. A raw JS setTimeout
// registration is invisible to that detector, exactly like the Promise a
// real fetch resolves through. See docs/PLAN.md.
func asyncCall(fn func()) {
	var jsFn js.Func
	jsFn = js.FuncOf(func(this js.Value, args []js.Value) any {
		jsFn.Release()
		fn()
		return nil
	})
	js.Global().Call("setTimeout", jsFn, 0)
}

// deadlockChild's Init blocks on a channel only a LATER async callback can
// fill — the exact shape of webtyp.com/view's callerLister.list() and
// appointment_booking's own reservationFormStore.List(): both do
// `ch := make(chan error, 1); caller.Call(...); <-ch` inside Init()/Reload().
// done closes the instant this Init() actually completes, whenever that
// turns out to be.
type deadlockChild struct {
	Element
	done chan struct{}
}

func (c *deadlockChild) Init(_ Ctx) {
	ch := make(chan struct{}, 1)
	asyncCall(func() { ch <- struct{}{} })
	<-ch
	close(c.done)
}

func (c *deadlockChild) Render() *Element {
	return NewElement("div").ID(c.GetID()).Text("mounted")
}

// trigger mounts deadlockChild from INSIDE an already-in-flight async
// callback — the exact shape of BookingView.reloadServices: two
// caller.Call()s resolve, and the callback that runs once both are done
// calls rebuildPanel(), which does panel.Set(...) — a SignalNodes.Set from
// inside a real fetch's resolution handler, never from the composition root.
type trigger struct {
	Element
	slot *SignalNodes
	done chan struct{}
}

func (t *trigger) Init(_ Ctx) {
	t.slot = NewNodes()
	t.done = make(chan struct{})
	asyncCall(func() {
		child := &deadlockChild{done: t.done}
		child.SetID("deadlock-child")
		t.slot.Set([]*Element{NewElement("div").Child(child)})
	})
}

func (t *trigger) Render() *Element {
	return NewElement("div").ID(t.GetID()).BindChildren(t.slot)
}

// TestReactiveMountFromAsyncCallback_DoesNotDeadlock reproduces the
// production crash documented in docs/PLAN.md: a component whose Init()
// blocks on a channel only a later async callback can fill deadlocks the
// ENTIRE wasm program — silently, with no panic text reaching the page — the
// moment it is mounted reactively (via Signal.Set) from inside an
// already-async callback instead of from the composition root's own
// synchronous startup.
//
// Pre-fix: this test hangs. dom's reconcile calls deadlockChild.Init()
// SYNCHRONOUSLY, nested inside trigger's own async callback; that Init()
// then blocks on a channel only ANOTHER async callback can fill, and
// nothing can ever run it because we are already mid-callback with no
// Go-visible timer pending — the js/wasm runtime detects zero runnable
// goroutines and silently calls runtime.wasmExit(0) (the real error is
// "fatal error: all goroutines are asleep - deadlock!", invisible outside a
// real Go stack trace — this app's own browser tooling never surfaces it).
// <-tr.done then blocks forever; gotest's own external per-package timeout
// (30s default) is what turns that into an observable failure instead of a
// false green — do NOT add an internal time.After/select fallback here, for
// the same reason asyncCall avoids Go timers: it would mask the bug.
//
// Post-fix: dom defers a reactively-mounted component's Init() to its own
// microtask instead of running it nested inside the triggering callback, so
// deadlockChild's blocking wait runs on a clean call stack and resolves
// normally. <-tr.done returns quickly and the test passes.
func TestReactiveMountFromAsyncCallback_DoesNotDeadlock(t *testing.T) {
	domtest.Mount(t, "deadlock-root")

	tr := &trigger{}
	tr.SetID("deadlock-trigger")
	if err := Render("deadlock-root", tr); err != nil {
		t.Fatalf("Render: %v", err)
	}

	<-tr.done

	if got := domtest.Text("#deadlock-child"); got != "mounted" {
		t.Fatalf("want %q, got %q — deadlockChild never finished mounting", "mounted", got)
	}
}
```

Run it in isolation first, against the CURRENT unfixed code:

```bash
gotest -run TestReactiveMountFromAsyncCallback_DoesNotDeadlock -t 20
```

**Required outcome at this stage: the command fails (times out).** That failure — not a green
run — is the acceptance criterion for Stage 1. If it passes cleanly here, something about the
reproduction doesn't match production; stop and re-examine before proceeding (do not weaken or
remove the test to make it pass — the goal is a red test that the fix turns green).

## Stage 2 — implement the fix

Apply the `reconciling` flag + `deferToMicrotask` change described above, precisely at
`reconcileChildren`'s "create and insert" branch. Do not touch `Append`'s call to
`initComponent`, and do not change `initComponent` itself — the top-level composition-root
mount must remain exactly as synchronous as it is today.

## Stage 3 — verify

```bash
gotest -run TestReactiveMountFromAsyncCallback_DoesNotDeadlock -t 20   # must now PASS
gotest                                                                  # full suite, must be green
```

The full suite matters here specifically: deferring the insert-and-wire step changes *when*
(not *whether*) a reactively-mounted subtree lands in the DOM relative to the `Set()` call that
triggered it. Any existing test that reads DOM state synchronously right after a `Set()` call
targeting a *newly-appearing* node (not an update to an already-mounted one — those are
unaffected, they take reconcileChildren's other branches) will need either a fixed round-trip
wait (mirroring how this plan's own new test waits) or is itself relying on a timing guarantee
`docs/BINDING_MODEL.md` never actually made. Investigate and note any such case rather than
loosening this fix to make it pass.

## Documentation

- `docs/BINDING_MODEL.md` — after the existing "corre exactamente una vez, antes del Render()"
  line, add one sentence: a reactively-mounted component's `Init()`→`Render()`→DOM-insertion
  runs as its own microtask, decoupled from whatever callback triggered the mount, precisely so
  a blocking `Init()` (a normal, supported pattern — see the same doc's own guidance on data
  loading in `Init()`) never nests inside the async callback that scheduled it.
- `docs/TRADEOFFS.md` — if it lists known timing guarantees, add this one and the reasoning
  (matches an existing entry's shape if a "reactive update timing" section already exists;
  otherwise a short new entry is fine).

## Acceptance criteria

- [ ] `tests/uc_reactive_mount_deadlock_test.go` exists exactly as specified above.
- [ ] Confirmed (and noted in the PR description) that this test failed/timed out against the
      pre-fix code.
- [ ] `reconciling` field + `deferToMicrotask` fix applied exactly at `reconcileChildren`'s
      create-and-insert branch; `Append`/top-level `initComponent` call sites unchanged.
- [ ] `gotest -run TestReactiveMountFromAsyncCallback_DoesNotDeadlock` passes.
- [ ] `gotest` (full suite) is green.
- [ ] `go build ./...` (backend/SSR target) and `GOOS=js GOARCH=wasm go build ./...` both
      compile clean — this package has no TinyGo-only code path today; do not introduce one.
- [ ] `docs/BINDING_MODEL.md` updated per above.

| Stage | Task |
|-------|------|
| 1 | Add the reproduction test; confirm it fails against current code (red) |
| 2 | Implement the `reconciling` + `deferToMicrotask` fix |
| 3 | Re-run the new test (green) + full suite (green) + both build targets |
| 4 | Update `docs/BINDING_MODEL.md` (and `docs/TRADEOFFS.md` if applicable) |
