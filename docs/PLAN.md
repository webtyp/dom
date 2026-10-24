---
PLAN: "fix!: BindChildren owns the element's children — same output on server and browser; static Child beside it is rejected"
EXECUTOR: jules
REVIEWER: none
STATUS: review
SESSION: 13188905694099823682
PR: https://github.com/webtyp/dom/pull/31
REVIEWED_COMMIT: 31026adf662356fd4e286c4ce0f88cb6108f6253
---

> Read AGENTS.md at the repo root before any change.

# PLAN — one rule for `BindChildren` on server and browser

## Why (the bug)

Today an element with `BindChildren(signal)` renders differently depending on where it runs
(`serialize.go`, `case "children"`):

- **server / stdlib (no observer):** only the static `Child(...)` content; the signal is ignored.
- **browser (observer present):** the signal's current nodes **plus** the static `Child(...)` content.

Consumers copied the signal's nodes into `Child(...)` so the server output was not empty
(`webtyp/layout/platformd/platformd.go`, 5 places; `webtyp/components/datatable/table.go`). In the
browser the same nodes are then written twice, and `claimID` panics:

```
panic: dom: id 10 was written twice in one render, by <button> and <button> — a single component instance is being rendered in two places …
```

Reproduced 2026-10-24: any `platformd` app whose start screen has a description (`platformd.Describe`)
crashes on load (`veltylabs/service_catalog` demo).

**The rule after this plan:** an element with `BindChildren` shows exactly what its signal holds —
on the server and in the browser. It has no `Child(...)` content of its own; writing both is a
programming error reported loudly. Consumers put their initial nodes in the signal (`NewNodes(...)`
or `Set(...)` before rendering) and never copy them.

## Tools

```
go install webtyp.com/devflow/cmd/gotest@latest
```

## Design gate

1. **Prior art:** React and Vue render the same markup on server and client from one source
   (hydration mismatches are errors); Svelte and SolidJS forbid mixing static children with a
   reactive `{#each}`/`<For>` owner of the same parent. This change aligns `dom` with all of them.
2. **Novice-name test:** no new names. The panic text says what to do in plain words.
3. **Complexity ledger:** concepts −1 (no «server-only seed» to remember); ways to give a bound
   list its first rows −1 (only the signal); lines at call sites −3 per seeded slot.
4. **Where it belongs:** in `dom`, which owns `BindChildren`; consumers stop patching around it.
5. **Deletes:** the `hasObserver` gate on `case "children"`; `TestSSRIgnoresBindChildren` (it
   asserts the old split). Consumers delete their seeding loops in their own plans (`layout`,
   `components`), after this tag.

## Stage 1 — `serialize.go`

1. Add, next to the other dom errors/messages in the package (same file is fine), the constant:

```go
// errChildrenBesideBinding is the panic text when an element has both BindChildren and
// static Child(...) content: its signal owns the children.
const errChildrenBesideBinding = "dom: this element uses BindChildren and also has Child(...) content — the signal owns the children: put the first nodes in the signal (NewNodes(...) or Set(...) before rendering) instead of Child(...)"
```

2. In `writeElement`, `case "children"`: delete the `if hasObserver {` gate and its comment, so the
   case becomes:

```go
		case "children":
			// The signal owns the element's children, on the server and in the
			// browser alike: both render exactly sig.Get().
			if sig, ok := bind.signal.(*SignalNodes); ok {
				boundChildren = append(boundChildren, sig.Get()...)
			}
			hasBoundChildren = true
```

   Declare `hasBoundChildren := false` next to `var boundChildren []*Element`.

3. Right after the `for _, bind := range el.bindings { … }` loop, add:

```go
	if hasBoundChildren && len(el.children) > 0 {
		panic(errChildrenBesideBinding)
	}
```

Nothing else in `serialize.go` changes.

## Stage 2 — tests (`tests/escaping_test.go`, package `tests`)

Delete `TestSSRIgnoresBindChildren` and its comment. Add:

```go
// An element with BindChildren renders exactly what its signal holds, also on
// the server: the same markup the browser shows before any update.
func TestBindChildren_ServerRendersTheSignal(t *testing.T) {
	el := NewElement("ul").BindChildren(NewNodes(NewElement("li").Text("first")))
	got := el.String()
	want := "<ul><li>first</li></ul>"
	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

// Static Child(...) beside BindChildren is a programming error: it used to
// render twice in the browser (claimID panic) and once on the server.
func TestBindChildren_StaticChildBesideItPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected a panic for Child(...) beside BindChildren")
		}
	}()
	_ = NewElement("ul").
		BindChildren(NewNodes(NewElement("li").Text("bound"))).
		Child(NewElement("li").Text("static")).
		String()
}
```

Use the same imports the file already has (dot-import of `webtyp.com/dom` if that is how the file
calls `NewElement`; copy the style of the surrounding tests). If another existing test in this repo
seeds `Child(...)` beside `BindChildren`, change it to put those nodes in the signal and list it in
the PR. Find them with: `grep -rn "BindChildren" --include=*_test.go .`

## Stage 3 — docs

- `README.md` line ~143 (`| .BindChildren(s *SignalNodes) | keyed child list |`): becomes
  `| .BindChildren(s *SignalNodes) | keyed child list; the signal owns the children (no Child(...) beside it) |`.
- `docs/BINDING_MODEL.md`, section `### Lista reactiva: BindChildren`: add one paragraph at its end:
  `La señal es dueña de los hijos: el servidor y el navegador dibujan exactamente lo que contiene. No se agregan hijos con Child(...) al mismo elemento (dom se detiene con un error que lo explica); las filas iniciales se ponen en la señal con NewNodes(...) o Set(...).`
  (This file is already in Spanish; keep it so.)

## Delivery check

```
GOPRIVATE=webtyp.com/*,github.com/veltylabs/* GOFLAGS=-mod=mod gotest
```

- A PR that does not compile, or whose tests could not run, is a failed delivery.
- Never pause to ask permission or report progress: when the check passes, commit, push and open
  the PR. Stuck for 30 minutes: open the PR anyway listing what fails.
- Commit after every stage (`git add -A` then `git commit -m "stage N: …"`); never `git reset`,
  `git restore`, `git checkout -- <file>` or `git stash` on your own work.
- `git diff --stat main` must list every file below and nothing the plan did not order.

## Stages

| # | Files | Done when |
|---|---|---|
| 1 | `serialize.go` | gate removed, panic on both |
| 2 | `tests/escaping_test.go` (+ any test seeding beside `BindChildren`, listed in the PR) | both new tests pass, old one deleted |
| 3 | `README.md`, `docs/BINDING_MODEL.md` | text added |
