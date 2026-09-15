//go:build wasm

package dom_test

import (
	"testing"

	. "webtyp.com/dom"
	"webtyp.com/dom/domtest"
)

// replacer swaps the single node in its slot for a DIFFERENTLY-keyed node,
// from inside an async callback — the exact shape of
// appointment_booking's BookingView.rebuildPanel replacing its "Elija un
// profesional…" placeholder with the real panel once the services call
// resolves. The replacement carries no Init() at all: this is about the
// reconcile's own bookkeeping, not about blocking.
type replacer struct {
	Element
	slot *SignalNodes
	done chan struct{}
}

func (r *replacer) Init(_ Ctx) {
	r.slot = NewNodes(NewElement("div").ID("replace-old").Text("old"))
	r.done = make(chan struct{})
	asyncCall(func() {
		r.slot.Set([]*Element{NewElement("div").ID("replace-new").Text("new")})
		// Two further macrotask hops: the reconcile's own deferred work is a
		// microtask, so it has certainly drained by the time these run.
		asyncCall(func() { asyncCall(func() { close(r.done) }) })
	})
}

func (r *replacer) Render() *Element {
	return NewElement("div").ID(r.GetID()).BindChildren(r.slot)
}

// TestReactiveReplace_RemovesStaleNode guards the half of reconcileChildren
// that its deferred-insert path must not desynchronise: the "remove extra
// nodes" loop compares the parent's LIVE child count against len(newNodes).
// If insertion is deferred but removal is not, that comparison runs against
// a DOM that still holds only the OLD children — for a 1-for-1 replacement
// it reads 1 > 1, removes nothing, and the deferred insert then appends
// alongside the node it was supposed to replace. Both end up on screen.
func TestReactiveReplace_RemovesStaleNode(t *testing.T) {
	domtest.Mount(t, "replace-root")

	r := &replacer{}
	r.SetID("replace-host")
	if err := Render("replace-root", r); err != nil {
		t.Fatalf("Render: %v", err)
	}

	<-r.done

	host, ok := Get("replace-host")
	if !ok {
		t.Fatal("host element not found")
	}
	_ = host

	if got := domtest.Text("#replace-new"); got != "new" {
		t.Fatalf("replacement node: want %q, got %q", "new", got)
	}
	if got := domtest.Text("#replace-old"); got != "" {
		t.Fatalf("stale node still in the DOM with text %q — the replaced node was never removed", got)
	}
}
