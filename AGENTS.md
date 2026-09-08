# Agent Guide — `webtyp/dom`

Constraints and rules for agents adding or modifying functionality in this package.
Read this before making any change.

---

## Construction Harness — typed & explicit (the WebTyp approach)

`dom` is the **source** of WebTyp's construction harness: the typed, explicit API is what keeps an
agent that doesn't know the library from building wrong code. Every API you add here must uphold it:

- **Typed over `any`** — no generic slots. The builder is typed methods (`Text`/`Child`/`Attr`/
  `Class`/`Set(...fmt.KeyValue)`), like `webtyp/json`'s writer, **reusing `fmt` types** (no new
  types); `Add(...any)` is removed. Reactive content goes ONLY through a signal binding
  (`BindText`/`Bind*`), which requires a `*Signal*`.
- **Explicit names** — `Text` (static) vs `BindText` (reactive); reading a call states intent.
- **Illegal states unrepresentable; fail at compile time.** What the compiler can't catch becomes a
  `devMode` warning (nil signal, `*Element` embed, bad list keys, `Bind` on non-input) — **never** a
  silent failure.
- **Minimal public surface** — export only what an author types; engine plumbing (`update`,
  `subscribe`, internal interfaces) stays unexported.
- **Docs are minimal "how" instructions, not long skills** — if a rule must be *remembered*, close
  it with types, not prose.

(Ecosystem rationale: `webtyp/app/docs/CONSTRUCTION_HARNESS.md`.)

---

## Fundamental Constraint

**`webtyp/dom` is the only package in the webtyp ecosystem that may import `syscall/js`.**

Any other package (`webtyp/components/*`, apps, etc.) that needs browser APIs must call public functions from `dom`. Never add `syscall/js` imports outside this package.

---

## Package Pattern: Free Functions, No Types

The established pattern is **free public functions**, not methods on a custom type.

```
✅ SetHash(hash)     LocalStorageGet(key)     SetDocumentAttr(attr, value)
❌ h.SetHash(hash)   ls.Get(key)              doc.SetAttr(attr, value)
```

- No `Handle` type. No receiver types wrapping DOM state.
- No sub-packages inside `dom`.
- New APIs must follow the same flat function style as `SetHash`/`GetHash`, `LocalStorageGet`/`LocalStorageSet`, `SetDocumentAttr`/`GetDocumentAttr`.

---

## Build Split Rules

| API category | `//go:build wasm` file | `//go:build !wasm` stub |
|---|---|---|
| Called only from `*_wasm.go` / WASM-only callers | ✅ Required | ❌ None — omit the stub |
| Called from code without a build tag (e.g. inside `Render()`) | ✅ Required | ✅ Required — no-op / `""` |

**Examples:**
- `LocalStorage*` → only called from WASM code → **no backend stub**.
- `SetDocumentAttr`, `GetDocumentAttr` → called from `ThemeSwitch.Render()` (no build tag) → **both files required**.
- Backend stubs return `""` or are no-ops. Never keep in-memory state in a backend stub — concurrent server requests would share it and contaminate each other.

---

## Error Handling in TinyGo WASM

**`defer/recover` does NOT work in TinyGo WASM.**

> On architectures where `recover` is not implemented, a panic always exits the program without running deferred functions.

Use O(1) guards instead:

```go
// ✅ Guard before any JS call
if !LocalStorageAvailable() {
    return "", Err("localStorage unavailable")
}

// ❌ Never rely on defer/recover to catch JS panics
defer func() { recover() }()  // silently does nothing in TinyGo
```

For quota management, maintain an in-memory counter (see `lsUsedBytes` in `domWasm`) and validate before calling JS, not after.

---

## Zero Standard Library

Never import `fmt`, `strings`, `errors`, or other stdlib packages.
Use `webtyp.com/fmt` for formatting and error construction:

```go
import . "webtyp.com/fmt"

return Err("localStorage unavailable")
return Errf("value too large for key %s", key)
```

---

## Slices Over Maps

Maps are extremely heavy in TinyGo. Use `[]fmt.KeyValue` for attributes and events — not `map[string]string`.

---

## The `""` = Absent Convention

Throughout the package, an empty string means "absent" or "remove":

- `LocalStorageGet` returns `("", nil)` for a missing key (not an error).
- `SetDocumentAttr(attr, "")` removes the attribute.
- `GetDocumentAttr` returns `""` if the attribute is absent.
- `GetHash()` backend returns `""` — server has no URL state.

New APIs must follow this same convention.

---

## Internal State: Singleton Fields, Not Package Variables

All browser-side state lives in `domWasm` fields (`document`, `localStorage`, `lsUsedBytes`), not in package-level variables. This keeps browser state centralized in the singleton and avoids shared mutable globals.

---

## DOM Boundaries

`dom` is a **JS bridge** — it exposes DOM primitives without business logic.

- `SetDocumentAttr("data-theme", "dark")` is the same primitive as `SetDocumentAttr("lang", "es")`. Semantic meaning (which values are valid, when to rotate them) lives in the component that calls it.
- Do not add theme logic, routing logic, or form logic to `dom`. Those belong in `themeswitch`, `router`, `form`, etc.

---

## Adding a New Browser API

Checklist before opening a PR:

1. **Does it require `syscall/js`?** → it belongs in `dom`. If not, reconsider the location.
2. **Is it called from tag-less code?** → add a `!wasm` stub (no-op / `""`).
3. **Does it only run in the browser?** → no stub needed; mark the file `//go:build wasm`.
4. **Error path**: use O(1) guards, no `defer/recover`.
5. **Naming**: full words, no abbreviations (`LocalStorageDel` not `LsDel`).
6. **No new types or sub-packages.**
7. **Tests**: real browser tests in `dom/tests/uc_<name>_test.go` (`//go:build wasm`, `package dom_test`). Run with `gotest`.

---

## Testing

Tests that exercise browser APIs run in a real browser via `gotest`:

```bash
go install webtyp.com/devflow/cmd/gotest@latest
gotest
```

- Public API tests → `dom/tests/uc_*_test.go` (`package dom_test`)
- Tests requiring internal access → root of package (`package dom`)
- All new browser API tests go in `dom/tests/`, not in the package root.
- WASM tests use `webtyp.com/dom/domtest`; do not hand-roll a mount root or query/event helpers.
- Each test cleans up after itself: call `LocalStorageClear()` and `SetDocumentAttr("data-theme", "")` in cleanup, ignoring the returned error.

---

## Mount Points

```go
✅ Render("app", &App{})    // correct — sprite SVG stays intact
❌ Render("body", &App{})   // destroys the SVG sprite injected by sitec
```

---

## Component Contract — ONE way (signals)

A component implements **only** `Render() *Element` (+ optional `Init(ctx dom.Ctx)` that runs ONCE
before first render). There is **no** `OnMount`/`OnUpdate`/`OnUnmount` and **no** manual `Update()`
(it is unexported). Dynamic state lives in **typed signals**; changing a signal patches only the
bound DOM node — never re-render a whole element (no Virtual DOM).

```go
✅ type Counter struct { dom.Element; n *dom.SignalString }   // value-embed Element; state in a signal
❌ type Counter struct { *dom.Element; count int }             // pointer embed + plain field + Update()
```

- **Events** are declared in `Render()` via `.On(event, handler)`; the handler only mutates signals.
- **Init** is for one-time setup (load storage, fetch, subscribe). Set signals here — even from a
  goroutine; the bound DOM patches directly. Register teardown with `ctx.OnCleanup(fn)`.
- Embed `dom.Element` **as a value**, never as a pointer.
- Inside `Render()`, assign identity via `.Key("key")` and resolve the live DOM handle via `.Ref()`; never invent global explicit IDs inside components.

## No Generics

The ecosystem uses **zero** generic functions and follows the `webtyp/fmt` codec rule
(*"cero any, cero map"*) — typed methods per primitive. Signals are concrete typed cells, not
`Signal[T]`. The DOM boundary is `string`/`bool`, so:

- `SignalString` / `SignalBool` / `SignalNodes` (+ `NewString`/`NewBool`/`NewNodes`);
  `Get`/`Set`, and `Toggle()` on `SignalBool`.
- Bindings (raw signal): `BindText`, `BindAttr`, `BindClass`, `BindAttrBool`, `BindState` (widget states — the ONLY sanctioned writer; `BindAttrBool` on `data-*` is the targetlist defect), `Bind` (two-way input),
  `BindChildren` (keyed list of `*Element`), `Key`, `Autofocus`; `Show` for conditionals.
- Bindings (computed): `BindTextFunc`/`BindAttrFunc`/`BindClassFunc`/`BindAttrBoolFunc`/`BindStateFunc` take a
  function and **auto-track** the signals it reads — no dependency list. `DeriveString`/`DeriveBool`
  for a named shared computed value.

If a numeric display is needed, format to `string` at the component. Never introduce `[T any]`.

## Public vs Internal Surface — keep the API clean

**Public = exactly what a component author types.** Engine plumbing stays unexported. Unexport any
symbol that only one package uses (constants included).

- `subscribable` is **unexported** (its method `subscribe` is unexported → only `dom`'s signals
  satisfy it; authors pass concrete signals as `...subscribable` without naming it).
- `initable` is **unexported** but its method `Init` is exported (engine asserts
  `component.(initable)`; the author only writes `func Init(ctx dom.Ctx)` and the public `Ctx`).
- `update` (formerly `Update`) is **unexported** — used only by `Show`/`BindChildren`. Authors
  cannot call it: "no manual update" is enforced at compile time.
- Signal struct fields (`v`, `subs`) stay unexported; access via `Get`/`Set` only.

---

## Reference

- `docs/ARCHITECTURE.md` — full API reference, build split, rendering patterns.
- `web/client.go` — canonical usage example.
