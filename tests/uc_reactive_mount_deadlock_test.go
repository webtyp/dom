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
