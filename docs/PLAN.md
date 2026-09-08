---
PLAN: "feat(dom): Element.Ref() — the typed bridge from a built element to its live node"
TAG: v0.13.11
EXECUTOR: jules
REVIEWER: none
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# PLAN — `Element.Ref()`: the missing contract

## The defect

`dom` has **no typed way for a component to reach the live DOM node of an element
it built itself**. The only way to obtain a `Reference` in the entire package is
`Get(id string)` — verified: there is no `*Element` → `Reference` bridge anywhere.

So an author who needs their own node back is **forced to invent a global id**:

```go
// today — the only path the API offers
func (c *Comp) Render() *Element {
    return NewElement("div").Child(NewElement("span").ID("stc-row"))
}
func (c *Comp) later() { row, ok := Get("stc-row") } // a name the author made up
```

That invented name is a global. Two instances of the same component on one page
write it twice and `claimID` panics — the `app-demo` bug (two `calendarslider`s
emitting `id="cs-m-2026-09"`).

**The duplicate id is the symptom. The missing contract is the defect.**

### Evidence this is a library defect, not consumer sloppiness

- `dom`'s own consumer-shaped tests reach for `ID()`-inside-`Render()` in ~25-30
  places (`tests/uc_state_test.go`, `tests/uc_bindings_test.go`,
  `tests/escaping_test.go`, `lifecycle_wasm_test.go`,
  `show_regression_wasm_test.go`). The library's own authors, with full context,
  took that path every time — because it is the only one.
- The existing workaround, `Get(el.GetID())` (used by `components/usermenu`),
  **fails silently**: `GetID()` mints and caches an id, but if `dom` never
  serialized that id — an element with no events, bindings or autofocus gets no
  id in the WASM pass — `Get` returns `false` with no diagnostic. This was hit
  for real while fixing `calendarslider`: the month card could not be resolved
  and the scroll had to be re-anchored onto a button that happened to carry a
  click handler.

Per `CONSTRUCTION_HARNESS.md`: *"A missing contract at a boundary is a defect in
the library, not in the consumer"*, and principle 6 orders failure modes
*compile error → loud diagnostic → (never) silent failure*. The silent `false`
above is the forbidden category, and it exists in `dom` today.

## What was tried and rejected — do not rebuild it

PR [webtyp/dom#23](https://github.com/webtyp/dom/pull/23) was **closed without
merging**. It made `dom` *silently rewrite* author ids during serialization
(`sanitizeChildIDs`). Verified failures on that branch:

1. **SSR became non-deterministic** — the same tree serialized twice emitted
   different ids (`id='1'`, then `id='2'`), reintroducing exactly the bug the
   comment at `serialize.go:24-31` guards against.
2. **It mutated the author's `*Element` in place** (`"email"` → `"5"`),
   corrupting the value-embedded, built-once trees this repo documents as
   canonical.
3. **The id was not discarded** — it resurfaced as a `data-key` attribute on
   every keyed element.
4. `reconcileChildren` silently switched its reconciliation key from `id` to
   `data-key`.

**Do NOT implement any of these:** `sanitizeChildIDs`, `updateForAttr`, a
`data-key` attribute, `GetByKey`, or any change to `reconcileChildren`. A silent
rewrite is the one mechanism `CONSTRUCTION_HARNESS.md` names as never
acceptable.

## Scope of THIS plan — additive only

This plan **adds the missing contract and nothing else**. It does not forbid
`ID()`, does not panic, does not migrate anything. Reason: `ID()`-inside-`Render()`
is currently used by `webtyp/form`, `webtyp/layout` (`rightpanel`, `platformd`)
and six components. Enforcement is only possible once every one of them has a
typed path to move to — which is what this plan creates. Enforcement is a
separate, later plan.

**Nothing existing may break. Every current test must pass untouched.**

## Design gate

**1. Prior art.**
- *React* — `useRef` / `ref={}`: the author holds a handle to the node, never
  names it. Exactly the shape adopted here; we differ only in that our handle is
  the builder value itself (`*Element`), so no extra hook type is needed.
- *Svelte* — `bind:this={el}`: same idea, the element binds itself into a
  variable. Again a handle, not a name.
- *Lit* — `ref()` directive plus shadow DOM scoping. We have no shadow DOM (SSR +
  TinyGo budget), so scoping must come from dom-assigned ids instead.

Every one of them gives a **handle**, never asks the author for a **name**. `dom`
is the outlier, and that is the gap being closed.

**2. Novice-name test.** `el.Ref()` reads as a junior would say it: *"give me the
reference to that element."* No new noun is introduced — `Reference` already
exists and is already what `Get` returns.

**3. Complexity ledger.**

| | +N | −M |
|---|---|---|
| Concepts | 0 (`Reference` and `Key` both already exist) | 0 |
| Exported API | +1 method (`Ref`) | 0 (this plan) |
| Files | 0 | 0 |
| Call-site lines | +1 condition in an existing `if` | 0 |
| Ways to do it | **+1 temporarily** | −1 in the follow-up plan that removes `ID()`-in-components |

The "ways to do it" row is positive **only for the duration of the migration**,
and that is deliberate: the harness cannot forbid the old path before the new one
exists. The follow-up enforcement plan is what closes it to −1.

**4. Where it belongs.** `element.go` (the method, next to `GetID`) and
`serialize.go` (the id guarantee, one clause added to the auto-id `if` that is
already there). No new file, no new package.

**5. What it deletes.** Nothing yet — see the ledger note.

## Anti-footguns

- **Keep the comment at `serialize.go:24-31`** and the `hasObserver` gate it
  describes. SSR determinism depends on it. The new clause goes **inside** that
  gate — it must never make SSR emit ids.
- **Keep `claimID`** exactly as it is: behaviour and message unchanged.
- **No `map`** anywhere (TinyGo binary budget) and **no standard library** — use
  `webtyp/fmt`.
- `idsInPass` is a linear scan. Do not make every element addressable — only
  keyed ones — or that scan becomes O(n²) over the whole tree.
- `dom_frontend.go` is `//go:build wasm`; `dom_backend.go` is `//go:build !wasm`;
  `element.go` / `serialize.go` / `dom.go` build for both. `Ref` must compile on
  both.

## Stage 1 — `serialize.go`: a keyed element is addressable

`Key()` is already the author's identity contract (`BindChildren` reconciles on
it). Make it also the opt-in for "I will want this node back". One clause, inside
the existing observer-gated block:

```go
	if hasObserver {
		if (len(el.events) > 0 || len(el.bindings) > 0 || el.autofocus || el.key != "") && el.id == "" {
			el.id = generateID()
		}
		obs = observer[0]
		obs(el)
	}
```

Extend the comment above that block with one sentence: a keyed element is
addressable by its owner through `Ref()`, so it needs an id in the live DOM for
the same reason a bound one does. **Still gated on `hasObserver`** — SSR emits no
id and `(*Element).String()` stays byte-identical across renders.

## Stage 2 — `element.go`: `Ref()`

Add next to `GetID` (around `element.go:249`):

```go
// Ref returns the live DOM node this element was rendered into.
//
// It is the typed alternative to inventing a global id and calling Get on it:
// the author keeps the *Element they built and asks it for its node, so no name
// is chosen, and two instances of one component cannot collide.
//
//	func (c *Comp) Render() *Element {
//		c.row = NewElement("span").Key("row")
//		return NewElement("div").Child(c.row)
//	}
//	func (c *Comp) onSomething() {
//		if row, ok := c.row.Ref(); ok { row.SetText("hi") }
//	}
//
// ok is false before the element has been rendered, and for an element dom
// never gave an id — give it a Key to make it addressable. On the backend
// (SSR) there is no live DOM and Get's stub answer is returned unchanged.
func (b *Element) Ref() (Reference, bool) {
	if b.id == "" {
		return nil, false
	}
	return Get(b.id)
}
```

Do **not** call `GetID()` here: `GetID` mints an id, and minting one after
serialization produces an id no DOM node carries — the silent failure this plan
exists to remove. `Ref` reports `false` instead.

## Stage 3 — tests

`element_test.go` or a new `ref_test.go` (`//go:build !wasm`):

- **`TestRef_BeforeRender_False`** — a fresh `NewElement("span").Key("k")` has no
  id yet, so `Ref()` returns `ok == false`. No panic.
- **`TestSSR_KeyEmitsNoID`** — `NewElement("div").Key("k").String()` contains no
  `id=`, and serializing the SAME element twice is **byte-identical**. This is
  the regression for the closed PR's worst defect: keys must not leak ids into
  SSR.

`tests/uc_ref_test.go` (new, `//go:build wasm`, `package dom_test`) — the
consumer-shaped proof `CONSTRUCTION_HARNESS.md` requires:

- **`TestRef_ResolvesKeyedElement`** — a component whose `Render()` builds
  `NewElement("span").Key("row").Text("a")` (no events, no bindings — the case
  that silently failed before), keeps it in a field, and after `Render(...)`
  resolves it with `Ref()`; `SetText("b")` through the returned `Reference` must
  change the live DOM.
- **`TestRef_TwoInstances_DoNotCollide`** — two instances of that same component
  mounted as siblings in ONE render. Neither names an id, both mount with no
  panic, `Ref()` on each resolves a **different** node, and writing through A's
  reference leaves B's text untouched. This is the `app-demo` two-`calendarslider`
  scenario, reduced.

`gotest ./...` must be green **without editing any existing test** — this plan is
additive. If an existing test changes, the change is wrong.

## Stage 4 — documentation

- **`README.md`** — in the component section, the 6-line example from the `Ref`
  doc comment: keep the `*Element`, give it a `Key`, call `Ref()`.
- **`docs/ARCHITECTURE.md`** — a short subsection: ids are dom's; the author's
  identity is `Key`; `Ref()` is how a component reaches its own node. State that
  `ID()` remains for elements the app or chassis declares outside any component
  (`"app"`, `"body"`, external-CSS hooks) and that using it inside a component is
  what makes two instances collide.
- **`AGENTS.md`** — one bullet in the component-contract list, next to *"Embed
  `dom.Element` as a value, never as a pointer"*: inside `Render()`, use
  `Key()` + `Ref()`; never invent a global id.

Do **not** write that `ID()` is forbidden or that ids are discarded — neither is
true yet. Describe `Ref()` as the path to prefer.

## Acceptance criteria

- `grep -rn 'sanitizeChildIDs\|updateForAttr\|data-key\|GetByKey' .` → **empty**.
- `grep -n 'making SSR output for any bound element non-deterministic' serialize.go`
  → **1 hit** (the guard comment survived).
- `git diff --stat` touches no pre-existing `_test.go` file.
- `gotest ./...` green, `wasm ✅` and `race ✅` included.
- `gofmt -l .` → **empty**. `go vet ./...` → clean.
- `GOOS=js GOARCH=wasm go build ./...` OK.

## Out of scope

- Forbidding / panicking on `ID()` inside a component — a later plan, once
  `form`, `layout` and the components have migrated to `Ref()`.
- Any consumer repo.
- `claimID`, `reconcileChildren`, `data-key`.

## Stages

| # | File | Change |
|---|---|---|
| 1 | `serialize.go` | `|| el.key != ""` in the observer-gated auto-id clause |
| 2 | `element.go` | `(*Element).Ref() (Reference, bool)` |
| 3 | `ref_test.go`, `tests/uc_ref_test.go` | 4 tests, incl. the two-instance consumer-shaped proof |
| 4 | `README.md`, `docs/ARCHITECTURE.md`, `AGENTS.md` | document `Key` + `Ref` as the path |
