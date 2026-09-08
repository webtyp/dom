---
PLAN: "feat(dom): domtest — the WASM test harness every consumer is currently re-inventing"
TAG: v0.13.12
EXECUTOR: jules
REVIEWER: none
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# PLAN — `dom/domtest`: stop every test module re-inventing the DOM bootstrap

## The defect

`dom` ships no test harness, so **every module that tests against a live DOM
writes its own**, and they all disagree:

| Need | `dom/tests` | `html/tests` | `components/calendarslider` | `svg/tests` |
|---|---|---|---|---|
| prepare a mount root | `setupBindRoot()` | `SetupDOM(t)` | its own `TestMain` | **missing** |
| read an element's text | `queryText(sel)` | — | — | — |
| get an element | — | `GetRef(id)` | `query(t,sel)` / `exists(sel)` / `domDoc()` | **missing** |
| dispatch an event | — | `TriggerEvent(id,type,val)` | `.Call("click")` inline | **missing** |

`html/tests` even declares its own `TestReferenceInterface` + `TestReference`
because it had nothing to return — a private re-declaration of `dom.Reference`,
which is already exported.

This is not cosmetic. `svg/tests/uc_selectsearch_test.go` calls `SetupDOM`,
`GetRef` and `TriggerEvent` — helpers that live in `html/tests`, a **different
module's test package**, so they are not importable. That file has not compiled
since the module split; it stayed invisible only because `svg/tests` pinned
`webtyp.com/dom v0.13.9`, a tag published under the old `github.com/tinywasm/dom`
path, which made every `go` command in that module fail first. Unpinning it
(`svg v0.3.7`) exposed the real breakage.

Per `CONSTRUCTION_HARNESS.md`: *"The glue is written once, in the library that
owns it. If every application would write the same wiring, that wiring belongs to
a piece — not to the applications."* and *"A consumer never re-creates a missing
symbol locally."* The wiring belongs here.

## Design gate

**1. Prior art.**
- *React Testing Library* — `render()` + `screen.getByText()`: the framework ships
  the mount + query harness; consumers never build a container by hand. Same
  split adopted here.
- *Vue Test Utils* — `mount()` returning a wrapper with `find`/`trigger`. We
  return `dom.Reference` instead of inventing a wrapper type, because
  `dom.Reference` already is that type.
- *Playwright / Testing Library queries* — locate by CSS selector, act, assert.
  `Query`/`Fire`/`Fill` mirror that vocabulary.

**2. Novice-name test.** `domtest.Mount(t, "root")`, `domtest.Query(".row")`,
`domtest.Text(".row")`, `domtest.Fire(".btn", "click")`,
`domtest.Fill("#name", "Ana")` — each reads as the sentence a junior would say.

**3. Complexity ledger.**

| | +N | −M |
|---|---|---|
| Concepts | 0 (`Reference` already exists and is what `Query` returns) | 0 |
| Packages | +1 (`domtest`, wasm-only, test-only) | 0 |
| Exported API | +5 functions | **−2 helpers + −2 types** deleted from `dom/tests` and, in follow-up plans, ~8 more across `html/tests`, `svg/tests`, `components` |
| Ways to do it | +1 now | **−4** once the consumers migrate (one bootstrap instead of four) |

The last row ends negative: four ad-hoc bootstraps collapse into one.

**4. Where it belongs.** A new `domtest/` directory in this repo — `dom` owns the
live DOM, so it owns the harness for testing against one. It must be a **real
package** (`domtest/domtest.go`), not a `_test.go` file, or it stays unimportable
— which is the whole defect.

**5. What it deletes.** `setupBindRoot` and `queryText` in `dom/tests` (this
plan). `SetupDOM`, `GetRef`, `TriggerEvent`, `TestReferenceInterface` and
`TestReference` in `html/tests`, and the ad-hoc helpers in `components` and
`svg/tests` — follow-up plans in those repos.

## Anti-footguns

- `domtest` is **`//go:build wasm`** — it imports `syscall/js`. It must never be
  imported by non-test production code; nothing in this repo outside `*_test.go`
  may import it. Because Go links only what is imported, a test-only package adds
  **zero bytes** to an application's WASM binary — do not "optimise" it away.
- **Return `dom.Reference`, never a new interface.** `html/tests` inventing
  `TestReferenceInterface` is one of the defects being removed; do not repeat it.
- `Query` takes a **CSS selector**. `dom.Get` takes an **id** and stays exactly as
  it is — these are different lookups, not two ways to do one thing. Do not add a
  selector lookup to `dom` itself.
- **No `map`** in anything that could reach a WASM binary, and **no standard
  library** in `dom` proper — `domtest` may use `testing` and `syscall/js`, which
  are test/WASM-edge only.
- Do not touch `claimID`, `serialize.go`, `Ref()` or anything shipped in v0.13.11.

## Stage 1 — `domtest/domtest.go` (new package, `//go:build wasm`)

```go
// Package domtest is the shared harness for tests that drive a live DOM.
//
// Every module that renders into a browser needs the same four things: a clean
// mount point, a way to find a node, a way to read it, and a way to act on it.
// Before this package each one wrote its own, they disagreed, and one of them
// (svg/tests) referenced another module's unexported test helpers and stopped
// compiling. This is that wiring, written once, where dom owns it.
//
// Build-tagged wasm and imported only by tests, so it contributes nothing to an
// application binary.
package domtest
```

Exactly five functions — no more:

```go
// Mount prepares a clean element with the given id in the page body and routes
// dom's log through t. Call it at the top of every test that renders.
func Mount(t *testing.T, id string)

// Query returns the first element matching a CSS selector.
func Query(selector string) (dom.Reference, bool)

// Text returns an element's textContent, or "" when the selector matches nothing.
func Text(selector string) string

// Fire dispatches a bubbling event of the given type at the first element
// matching selector. No-op when nothing matches.
func Fire(selector, eventType string)

// Fill sets an input's value and fires "input", the pair every form test needs.
func Fill(selector, value string)
```

Behaviour, ported from the existing helpers so nothing regresses:

- `Mount` — `getElementById(id)`; if present, clear its `innerHTML`; otherwise
  create a `div`, set the id, append to `body`. Then
  `dom.SetLog(func(v ...any) { t.Log(v...) })` (see `dom.go:140`), which is what
  `html/tests`' `SetupDOM` did.
- `Query` — `document.querySelector(selector)`; on null/undefined return
  `(nil, false)`. Resolve the found node to a `dom.Reference`: read its `id`
  attribute and return `dom.Get(thatID)`. **If the node carries no id there is no
  way to build a `dom.Reference` from outside `dom` — in that case return
  `(nil, false)` and say so in the doc comment. Do NOT export new internals of
  `dom` to work around it, and do NOT declare a local reference type.** Record
  the limitation in the PR description; closing it is a separate decision.
- `Text` — `querySelector` then `textContent`; `""` when nothing matches. (The
  old `queryText` returned the literal `"<not found>"`; `""` is the honest
  answer and callers assert on real text.)
- `Fire` — `new Event(type, {bubbles: true})` then `dispatchEvent`.
- `Fill` — set `value`, then `Fire(selector, "input")`.

## Stage 2 — migrate `dom`'s own tests

`dom/tests/uc_bindings_test.go` declares `setupBindRoot()` and `queryText()`;
`uc_state_test.go`, `uc_ref_test.go`, `uc_documentattr_test.go`,
`uc_localstorage_test.go` and `escaping_test.go` use them.

- Delete both helpers.
- Replace `setupBindRoot()` with `domtest.Mount(t, "bind-root")` at every call
  site (it now takes `t` — the enclosing test already has it).
- Replace `queryText(sel)` with `domtest.Text(sel)`. Where a call site asserted
  against `"<not found>"`, assert against `""` instead.

`grep -rn 'setupBindRoot\|queryText' .` → **empty** when done.

This migration is the proof required by `CONSTRUCTION_HARNESS.md`: *"An API is
not published until a consumer-shaped test, inside the library itself, proves
it."* If any call site becomes awkward, the signature is wrong — fix the
signature, do not bend the test.

## Stage 3 — tests for the harness itself

`domtest/domtest_wasm_test.go` (`//go:build wasm`):

- **`TestMount_ClearsExistingRoot`** — mount, write `innerHTML`, mount again,
  assert the root is empty.
- **`TestQuery_MissAndHit`** — a miss returns `(nil, false)`; after rendering an
  element with an id, a hit returns a usable `dom.Reference`.
- **`TestText_EmptyOnMiss`** — `Text(".nope") == ""`.
- **`TestFireAndFill`** — render an `<input>` with an `On("input", …)` handler,
  call `Fill`, assert the handler saw the value.

`gotest ./...` green, `wasm ✅` and `race ✅` included.

## Stage 4 — documentation

- **`README.md`** — a short "Testing" section: import `webtyp.com/dom/domtest`,
  the five functions, one 8-line example. Say it is wasm-only and test-only.
- **`AGENTS.md`** — one bullet in the testing guidance: WASM tests use
  `domtest`; do not hand-roll a mount root or a query helper.

Do not document the deleted helpers.

## Acceptance criteria

- `grep -rn 'setupBindRoot\|queryText' --include=*.go .` → **empty**.
- `head -1 domtest/domtest.go` → `//go:build wasm`.
- `grep -rn 'domtest' --include=*.go . | grep -v _test | grep -v '^./domtest/'`
  → **empty** (nothing outside tests imports it).
- `grep -rn 'TestReferenceInterface\|type TestReference' --include=*.go .` →
  **empty** in this repo.
- `gotest ./...` green, `wasm ✅` and `race ✅` included.
- `gofmt -l .` → empty; `go vet ./...` clean.
- `GOOS=js GOARCH=wasm go build ./...` OK.

## Out of scope

- `html/tests`, `svg/tests`, `components` — their own follow-up plans migrate to
  `domtest`; `svg/docs/PLAN.md` is already written and waiting on this one.
- Adding a selector lookup to `dom` proper. `Get(id)` stays as it is.
- Anything shipped in v0.13.11 (`Ref`, the keyed auto-id).

## Stages

| # | Files | Change |
|---|---|---|
| 1 | `domtest/domtest.go` | new wasm-only package: `Mount`, `Query`, `Text`, `Fire`, `Fill` |
| 2 | `dom/tests/*.go` | delete `setupBindRoot` / `queryText`, migrate every call site |
| 3 | `domtest/domtest_wasm_test.go` | 4 tests for the harness |
| 4 | `README.md`, `AGENTS.md` | document `domtest` as the one testing path |
