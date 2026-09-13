---
PLAN: "feat(dom): typed event registration and a keyboard contract"
EXECUTOR: unassigned
REVIEWER: none
---

> **Phase A (GATE)** of
> [`webtyp/docs/TYPED_EVENTS_AND_SURFACES_MASTER_PLAN.md`](https://github.com/webtyp/webtyp/blob/main/docs/TYPED_EVENTS_AND_SURFACES_MASTER_PLAN.md).
> Nothing in `components` or `widget` may migrate before this ships a tag.

# Plan — the event type stops being a string

Files touched: `element.go`, `event.go`, `event_wasm.go`, `dom_backend.go`,
`key.go` (new), `event_wasm_test.go`, `README.md`.

## 0. The api-design gate — five answers

### 1. Prior art

| Framework | How the event type is expressed | Handler parameter |
|---|---|---|
| **React** | one prop per event: `onClick`, `onKeyDown` | narrowed per event (`KeyboardEvent`), and in TS `e.key` is a typed union |
| **Svelte** | `on:click`, `on:keydown` — directive carrying the name | narrowed through `HTMLElementEventMap` |
| **Elm** | typed constructors: `onClick`, `onInput`, `on "keydown" decoder` | the decoder states what it reads |
| **TypeScript `lib.dom`** | `addEventListener(type, …)` **with a mapped type** | `addEventListener("keydwon", …)` is a compile error; the handler is narrowed to `KeyboardEvent` |
| **vugu** (Go) | typed handlers over `vugu.DOMEvent` | one event type |

The invariant across all five: **the event name is never a free string at the
call site, and the handler's parameter is narrowed by the event.** Even the
`addEventListener` shape — the one `dom` copies today — is type-safe in
TypeScript because a mapped type closes it.

**Why this ecosystem differs.** Go has no mapped types, and TinyGo makes a
generics-heavy encoding expensive. The ecosystem's own answer to exactly this
problem already exists: `webtyp/json` writes **one method per primitive**
(`String`, `Int`, `Bool`, `Object`, `Array`) instead of one `Write(any)`.
`CONSTRUCTION_HARNESS.md` names that "the house pattern". One method per event
is that pattern applied to events.

### 2. The novice-name test

```go
el.OnClick(func(e dom.Event) { … })
el.OnKeyDown(func(e dom.KeyEvent) {
    if e.Key() == dom.KeyArrowLeft { … }
})
```

Read aloud by a junior with no context: *"on click, do this"*, *"on key down, if
the key is arrow left, do this"*. No lookup needed. The `On` prefix is the word
this package already uses, so nothing new is invented. `Key` is the DOM's own
word (`KeyboardEvent.key`), not an invention like `Code` or `Which`.

### 3. The complexity ledger

```
Concepts the developer must learn   +2 (KeyEvent, Key)   / −1 (the raw event-type string)
Exported methods on Element         +7                   / −1        ← WORSE, on purpose
Files they must touch to do X       +0                   /  0
Lines at the call site              +0                   /  0   On("click", f) → OnClick(f)
Ways to do the same thing            0                   / −1   ← On() is deleted, not deprecated
```

**The row that gets worse is the method count**, and it is the price of the row
that matters: `On("keydwon", f)` compiles today, never fires, and reports
nothing. After this change it does not compile. Seven names is a closed,
enumerable set — the whole ecosystem uses exactly **six** event types today
(`click` 35, `change` 14, `input` 7, `blur` 2, `toggle` 1, `submit` 1) and
**zero** keyboard events, because keyboard handling is currently impossible.

### 4. Where it belongs

`webtyp/dom`. A DOM event is the DOM's concern and this package already owns
registration. A separate `webtyp/keyboard` was considered and **rejected**: it
would have to wrap or duplicate `Element.On` — *"Never wrap a library to fix its
behaviour. A wrapper that patches a defect is a fork with a friendlier name."*
The missing keyboard member is a defect **in `dom`**, not in its consumers.

### 5. What this change deletes

`func (b *Element) On(t string, h func(Event)) *Element` — deleted in this
change, not deprecated. All 47 non-test call sites migrate in their own repo's
phase. There is no version in which both paths exist.

## 1. What to build

### 1.1 `key.go` (new) — the typed key

`Key` is a string-backed named type so the zero value is meaningless rather than
a valid key, and the constants carry the DOM's own `KeyboardEvent.key` values
verbatim:

```go
type Key string

const (
	KeyArrowLeft  Key = "ArrowLeft"
	KeyArrowRight Key = "ArrowRight"
	KeyArrowUp    Key = "ArrowUp"
	KeyArrowDown  Key = "ArrowDown"
	KeyHome       Key = "Home"
	KeyEnd        Key = "End"
	KeyPageUp     Key = "PageUp"
	KeyPageDown   Key = "PageDown"
	KeyEnter      Key = "Enter"
	KeySpace      Key = " "
	KeyEscape     Key = "Escape"
	KeyTab        Key = "Tab"
)
```

Exactly these twelve. A key not in the set is reachable through the raw value a
`KeyEvent` still exposes; adding a constant is a one-line change here, in the
package that owns it — never a string literal at a call site.

### 1.2 `event.go` — narrow the contract, do not widen `Event`

`Key()` must **not** be added to `Event`. `Event` is what a click carries; a
`clickEvent.Key()` returning `""` is an illegal state made representable
(principle 3). Instead:

```go
// KeyEvent is what a keyboard handler receives. It is an Event plus the key.
type KeyEvent interface {
	Event
	Key() Key
}
```

### 1.3 `element.go` — one method per event

```go
func (b *Element) OnClick(h func(Event)) *Element
func (b *Element) OnChange(h func(Event)) *Element
func (b *Element) OnInput(h func(Event)) *Element
func (b *Element) OnBlur(h func(Event)) *Element
func (b *Element) OnSubmit(h func(Event)) *Element
func (b *Element) OnToggle(h func(Event)) *Element
func (b *Element) OnKeyDown(h func(KeyEvent)) *Element
```

Each delegates to the existing unexported registration path with its own literal
type string. The registration machinery does **not** change — only the door.
`On` becomes unexported (`on`).

### 1.4 `event_wasm.go` / `dom_backend.go`

`eventWasm` gains `Key() Key` reading `e.Get("key")`, guarded for
undefined/null like the existing accessors. The backend stub gains the same
method returning `""` — the backend never fires events.

## 2. The publication rule — the consumer-shaped test

> *An API is not published until a consumer-shaped test, inside the library
> itself, proves it.*

`event_wasm_test.go` gains a test that goes through the real stack a consumer
will use, with no doubles:

1. Render a real element tree with three focusable `<button>`s.
2. Register `OnKeyDown` on their container.
3. Dispatch a **real** `KeyboardEvent{key:"ArrowRight"}` via `syscall/js`.
4. Assert the handler received `dom.KeyArrowRight` **and** that a roving-focus
   implementation written against it moves `document.activeElement`.

Step 4 is the point: it is the exact thing `calendarslider` must do in Phase C.
If it is awkward to write here, the API is awkward to use, and the defect is
found before shipping.

## 3. Out of scope — named, not vague

`Event.TargetValue()`, `TargetID()` and `TargetChecked()` have the same shape of
defect: `TargetChecked()` on a click event is meaningless and returns `false`.
They are **not** fixed here because they are a different concern — a typed
*target*, not a typed *event kind* — and one plan doing two things is how both
get done badly.

This is a declared dependency, not a "known issue": it needs its own plan in
this repo before any consumer relies on a narrowed target. Do not paper over it
at a call site in the meantime.

## 4. Stages

| # | Stage | Files | Done when |
|---|---|---|---|
| 1 | The typed key | `key.go` | twelve constants, string-backed named type |
| 2 | The contracts | `event.go`, `event_wasm.go`, `dom_backend.go` | `KeyEvent` exists; `Key()` is NOT on `Event` |
| 3 | The door | `element.go` | seven `On*` methods; `On` unexported; nothing else changed |
| 4 | Consumer-shaped test | `event_wasm_test.go` | real key event moves real focus |
| 5 | Docs | `README.md` | the "I want X → use Y" table gains the events row; no prose restating what the signatures say |

`gotest` green at each stage. Publish the tag only after stage 5.

## 5. Zero technical debt — closing check

- `grep -rn "func (b \*Element) On(" .` returns nothing.
- `grep -rn 'On("' .` returns nothing in this repo.
- No `TODO`, no deprecation shim, no fallback that accepts a raw string.
- `README.md` does not restate a rule the signature already enforces.
