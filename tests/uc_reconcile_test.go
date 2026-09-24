//go:build wasm

package dom_test

import (
	"syscall/js"
	"testing"

	. "webtyp.com/dom"
	"webtyp.com/dom/domtest"
)

// Reconciliation contract tests: BindChildren must reconcile by Key, not by
// DOM id. Every row here carries Key("k-*") and NO explicit ID — the shape
// every target* widget builds (key "td-<id>", auto-minted numeric id). A test
// using explicit matching IDs exercises a path where id==key and cannot see
// this file's defects.

func recMount(t *testing.T, name string, rows ...*Element) *SignalNodes {
	t.Helper()
	root := name + "-root"
	domtest.Mount(t, root)
	t.Cleanup(func() {
		doc := js.Global().Get("document")
		if el := doc.Call("getElementById", root); !el.IsNull() && !el.IsUndefined() {
			el.Set("innerHTML", "")
		}
	})
	nodes := NewNodes(rows...)
	ul := NewElement("ul").ID(name + "-list").BindChildren(nodes)
	if err := Render(root, ul); err != nil {
		t.Fatalf("Render: %v", err)
	}
	return nodes
}

func recRow(key, text string) *Element {
	return NewElement("li").Key(key).Text(text)
}

func recKids(t *testing.T, name string) []js.Value {
	t.Helper()
	el := js.Global().Get("document").Call("getElementById", name+"-list")
	if el.IsNull() || el.IsUndefined() {
		t.Fatalf("list %q missing", name+"-list")
	}
	kids := el.Get("children")
	out := make([]js.Value, kids.Get("length").Int())
	for i := range out {
		out[i] = kids.Call("item", i)
	}
	return out
}

func recTexts(kids []js.Value) []string {
	out := make([]string, len(kids))
	for i, k := range kids {
		out[i] = k.Get("textContent").String()
	}
	return out
}

func assertRecTexts(t *testing.T, name string, want ...string) {
	t.Helper()
	got := recTexts(recKids(t, name))
	if len(got) != len(want) {
		t.Fatalf("%s: %d rows, want %d (%q)", name, len(got), len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: row %d = %q, want %q (full: %q)", name, i, got[i], want[i], got)
		}
	}
}

// Growth must keep every fresh row and drop every stale one. 2→3 where the
// third assertion names the defect: the count is right while the content is
// wrong — the exact user report (a duplicated row plus an unreachable one).
func TestReconcileGrowthKeepsFreshDropsStale(t *testing.T) {
	nodes := recMount(t, "rec-grow",
		recRow("k-a", "a"),
		recRow("k-b", "b"),
	)
	nodes.Set([]*Element{
		recRow("k-c", "c"),
		recRow("k-a", "a"),
		recRow("k-b", "b"),
	})
	assertRecTexts(t, "rec-grow", "c", "a", "b")
}

// Growth must reuse the nodes it already has, matched by key — not rebuild
// the world. Rebuilds drop focus, listeners and per-node state on every
// reload; the count test cannot tell, node identity can.
func TestReconcileGrowthReusesNodesByKey(t *testing.T) {
	nodes := recMount(t, "rec-reuse",
		recRow("k-a", "a"),
		recRow("k-b", "b"),
	)
	before := recKids(t, "rec-reuse")
	refA, refB := before[0], before[1]
	nodes.Set([]*Element{
		recRow("k-c", "c"),
		recRow("k-a", "a"),
		recRow("k-b", "b"),
	})
	after := recKids(t, "rec-reuse")
	if len(after) != 3 {
		t.Fatalf("3 rows, got %d", len(after))
	}
	if !after[1].Equal(refA) {
		t.Error("row k-a was rebuilt instead of reused (key match never hits)")
	}
	if !after[2].Equal(refB) {
		t.Error("row k-b was rebuilt instead of reused (key match never hits)")
	}
}

// Shrink must unmount exactly the dropped rows and keep the survivor's node.
func TestReconcileShrinkRemovesStaleKeepsSurvivor(t *testing.T) {
	nodes := recMount(t, "rec-shrink",
		recRow("k-a", "a"),
		recRow("k-b", "b"),
		recRow("k-c", "c"),
	)
	refA := recKids(t, "rec-shrink")[0]
	nodes.Set([]*Element{recRow("k-a", "a")})
	after := recKids(t, "rec-shrink")
	if len(after) != 1 {
		t.Fatalf("1 row, got %d", len(after))
	}
	if !after[0].Equal(refA) {
		t.Error("surviving row k-a was rebuilt instead of reused")
	}
	if after[0].Get("textContent").String() != "a" {
		t.Errorf("survivor text = %q, want %q", after[0].Get("textContent").String(), "a")
	}
}

// Reorder must move nodes, never rebuild them.
func TestReconcileReorderMovesNotRebuilds(t *testing.T) {
	nodes := recMount(t, "rec-reorder",
		recRow("k-a", "a"),
		recRow("k-b", "b"),
	)
	before := recKids(t, "rec-reorder")
	refA, refB := before[0], before[1]
	nodes.Set([]*Element{
		recRow("k-b", "b"),
		recRow("k-a", "a"),
	})
	after := recKids(t, "rec-reorder")
	if len(after) != 2 {
		t.Fatalf("2 rows, got %d", len(after))
	}
	if !after[0].Equal(refB) || !after[1].Equal(refA) {
		t.Error("reorder rebuilt nodes instead of moving them")
	}
	assertRecTexts(t, "rec-reorder", "b", "a")
}

// Same key, second render: the node is reused (identity). Content freshness
// on reuse is the WIDGET layer's contract (reactive row content, next stage)
// — at this layer the promise is only "same key, same node".
func TestReconcileSameKeyReusesNode(t *testing.T) {
	nodes := recMount(t, "rec-samekey", recRow("k-a", "a"))
	ref := recKids(t, "rec-samekey")[0]
	nodes.Set([]*Element{recRow("k-a", "a")})
	after := recKids(t, "rec-samekey")
	if len(after) != 1 {
		t.Fatalf("1 row, got %d", len(after))
	}
	if !after[0].Equal(ref) {
		t.Error("same key was rebuilt instead of reused")
	}
}

// Duplicate keys within one Set are author error (devMode warns); the
// reconciler must still behave deterministically — both rows mounted, in
// order, no panic — never silently drop one.
func TestReconcileDuplicateKeysDeterministic(t *testing.T) {
	nodes := recMount(t, "rec-dup", recRow("k-a", "a"))
	nodes.Set([]*Element{
		recRow("k-dup", "x"),
		recRow("k-dup", "y"),
	})
	assertRecTexts(t, "rec-dup", "x", "y")
}
