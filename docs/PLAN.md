---
PLAN: "feat(dom): OnUserActivity — one document-level presence signal"
EXECUTOR: jules
REVIEWER: none
STATUS: review
SESSION: 1959404247782613008
PR: https://github.com/webtyp/dom/pull/26
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.
>
> **Phase A (GATE)** of the idle-lock correction. Phase B (`webtyp/layout`:
> delete `platformd.armIdle` and consume this) cannot start until this ships a
> tag. Phase B is a separate plan in a separate repo — do NOT touch `layout`
> from here.

# Plan — `OnUserActivity`: the browser fact that a human is still there

## 0. Context — why this exists

`webtyp/layout/platformd` ships an idle lock (`IdleTimeout` + `OnIdle`: after N
seconds without activity the app logs out). It detects presence by hanging
element-level listeners on its own root:

```go
root.OnMouseEnter(func(Event) { p.activity() })
root.OnKeyDown(func(KeyEvent) { p.activity() })
root.OnFocusIn(func(Event) { p.activity() })
root.OnClick(func(Event) { p.activity() })
```

That is measurably wrong, and wrong in a way no consumer can fix from outside
`dom`:

- `mouseenter` fires **once** when the pointer crosses the element boundary. It
  does not repeat while the pointer moves. A user moving the mouse for thirty
  minutes without clicking is declared absent and logged out.
- `keydown` only reaches that root if focus is **inside** its subtree. Focus on
  `<body>` — the state after any click on chrome outside the shell — and no
  keystroke counts.
- Anything mounted **outside** that subtree (a portal, an overlay, a toast
  appended to the mount root) contributes no presence at all.

The consumer cannot repair this because presence is a **document** fact, not an
element fact, and `dom` is the only package in the ecosystem allowed to import
`syscall/js`. So the primitive belongs here.

`dom` already solved the identical shape once — `OnScrollCapture` exists because
`scroll` does not bubble, so a shell cannot wire it per element without knowing
other packages' internals. Read its doc comment in `dom.go` before writing code:
this new function is its sibling, and must read like it.

**Scope boundary.** `dom` reports *that the user did something*. It owns no
timer, no timeout, no "idle" concept, no logout. How long counts as idle and
what happens then stays with the consumer. Do not add any of that here — see
`AGENTS.md` → "DOM Boundaries".

## Design gate (api-design — five answers)

### 1. Prior art

| Library | Shape | Event set | Throttle |
|---|---|---|---|
| **react-idle-timer** | one component, `events` prop (overridable array), `element` prop defaults to `document` | `mousemove`, `keydown`, `wheel`, `mousedown`, `touchstart`, `touchmove`, `visibilitychange`, … (11 entries) | `eventsThrottle`, default 200 ms |
| **@ng-idle/core** | `Idle` service + `InterruptSource` abstraction; `DEFAULT_INTERRUPTSOURCES` is a document-level source | `keydown`, `mousemove`, `mousewheel`, `mousedown`, `touchstart`, `touchmove`, `scroll` | throttled inside the source |
| **idle-vue / Idle.js** | plugin, fixed document listener list | `mousemove`, `keydown`, `mousedown`, `touchstart` | none |
| **Browser-native** | `IdleDetector` (Idle Detection API) | — | — |

The invariant across all three libraries: **a document-level listener set, fixed
or defaulted, collapsed into one "the user did something" pulse, throttled
because `mousemove` fires at frame rate.** The browser-native `IdleDetector` is
Chromium-only and permission-gated, so it is not an option — every one of them
implements this by hand, and so do we.

We differ deliberately in shape: **one free function, no configuration, no
exposed event list.** The three libraries expose the event array because they
also own the timeout and must let the consumer tune sensitivity; `dom` owns
neither, so an `events []string` parameter would be a typo-prone string surface
(`.On(string, …)` was deleted from this package for exactly that reason — see
`README.md` → Events) buying nothing. The event set is an implementation
detail of "the user did something".

### 2. Novice-name test

`OnUserActivity(func(){ … })` reads as "on user activity, run this" — the same
sentence shape as the package's existing `OnHashChange` and `OnScrollCapture`.
The handler takes nothing and returns nothing because the only information it
carries is *that it happened*; a parameter would invite consumers to branch on
which input device moved, which is not a decision the shell should make.

### 3. Complexity ledger

| Axis | Δ |
|---|---|
| Concepts | **+1** (one function) / −0 |
| Files a consumer touches to detect presence | +0 / **−1** (`platformd` stops owning listener wiring, phase B) |
| Call-site lines | **+1** / **−13** (phase B deletes `armIdle`, the `idleArmed` field and its `Render()` call site) |
| Ways to do the same thing | +0 / −0 — and it removes the temptation to hand-wire per-element presence listeners, which is how the current bug arrived |

### 4. Where it belongs

`dom` is the JS bridge: it exposes browser primitives without business logic.
A document-level capture listener **is** a browser primitive and cannot live
anywhere else — `AGENTS.md` → "Fundamental Constraint": `dom` is the only
package that may import `syscall/js`. It is not a second concern in this
package: `OnHashChange` and `OnScrollCapture` already establish the
"page-level listener" category (`docs/ARCHITECTURE.md` → "Page-level
listeners"), and this is a third entry in it.

Explicitly rejected alternatives: a new `webtyp/idle` package (it would depend
on `dom` *and* `webtyp/time` and export two fields — a module, a go.mod and a
version to save zero concepts), and growing `webtyp/time` (it would have to know
about input events, i.e. depend on `dom`, inverting the dependency).

### 5. What it deletes

Nothing in `dom`. It unblocks the deletion in phase B of `platformd.armIdle`,
the `idleArmed` field, and the four element-level listeners quoted in §0.

---

## Stage 1 — the contract and the backend stub

### 1.1 `interface.dom.go`

Add to the `DOM` interface, immediately **after** the existing
`OnScrollCapture` entry (keep the page-level listeners adjacent):

```go
	// OnUserActivity registra listeners de presencia en el documento en fase de
	// captura: el handler se llama cuando el usuario hace algo en cualquier
	// parte de la página. Ver la función de paquete del mismo nombre.
	OnUserActivity(handler func())
```

Comment in Spanish, matching every other comment in that file. Comments in the
new `.go` code elsewhere follow whatever the surrounding file already uses.

### 1.2 `dom.go`

Add the package-level delegator immediately **after** `OnScrollCapture`, with
this doc comment verbatim (it is the contract; do not paraphrase):

```go
// OnUserActivity registra en el DOCUMENTO, en fase de captura, los listeners que
// significan "hay una persona ahí": movimiento y pulsación de puntero (ratón,
// dedo o lápiz), tecla, rueda y scroll. El handler se llama cuando ocurre
// cualquiera de ellos, en cualquier parte de la página.
//
// Existe por el mismo motivo que OnScrollCapture: la presencia es un hecho del
// DOCUMENTO, no de un elemento. Colgar los listeners del elemento raíz de un
// componente falla de tres formas — mouseenter se dispara UNA vez al cruzar el
// borde y no se repite con el movimiento, keydown solo llega si el foco está
// dentro de ese subárbol, y nada montado fuera del subárbol cuenta. Los tres
// fallos desaparecen en captura sobre el documento.
//
// El handler NO recibe nada: el único dato es que ocurrió. Se llama como mucho
// una vez por segundo — pointermove se dispara a la frecuencia de refresco y
// esto es una señal de presencia, no un flujo de eventos. Quien mida
// inactividad lo hace en segundos, así que la ventana no se nota.
//
// dom no sabe qué es estar inactivo: no hay temporizador, ni umbral, ni
// concepto de "idle" aquí. Cuánto tiempo sin actividad cuenta, y qué pasa
// entonces, es del consumidor.
//
// Registrar UNA sola vez: no hay forma de darlo de baja, son listeners del
// documento que viven lo que vive la página. Dos llamadas registran dos juegos
// de listeners, cada uno con su propia ventana de un segundo.
func OnUserActivity(handler func()) {
	instance.OnUserActivity(handler)
}
```

### 1.3 `dom_backend.go`

Add the no-op stub on the line **after** the existing `OnScrollCapture` stub:

```go
func (d *domBackend) OnUserActivity(handler func()) {}
```

A stub is **required** here, not optional: consumers call this from `Init(ctx)`,
which has no build tag and runs under SSR. See `AGENTS.md` → "Build Split
Rules". Keep no state in the stub — concurrent server requests share the
backend instance.

---

## Stage 2 — the WASM implementation

**File:** `dom_frontend.go` (`//go:build wasm`). Place the method directly
**after** `OnScrollCapture`, so the two page-level listeners stay together.

```go
// userActivityEvents es el juego de eventos que significan presencia. pointermove
// y pointerdown cubren ratón, dedo y lápiz con un solo listener cada uno — no hay
// mousemove/touchstart por separado. wheel está porque una página que ya está al
// final sigue emitiendo wheel sin emitir scroll: el usuario está ahí aunque nada
// se mueva.
//
// NO incluye visibilitychange: una pestaña visible no es una persona presente, y
// volver a ella no es actividad dentro de la página. Tampoco focus/blur de
// window, por lo mismo.
var userActivityEvents = []string{"pointermove", "pointerdown", "keydown", "wheel", "scroll"}

// userActivityThrottleMs es la ventana mínima entre dos llamadas al handler.
const userActivityThrottleMs = 1000

// OnUserActivity — ver la función de paquete del mismo nombre.
func (d *domWasm) OnUserActivity(handler func()) {
	// last es de ESTE registro, no del singleton: dos llamadas a OnUserActivity
	// deben tener ventanas independientes, o cada handler recibiría solo parte
	// de los pulsos. No es estado global mutable — muere con el listener.
	last := -float64(userActivityThrottleMs)

	// UN solo js.Func para los cinco eventos: comparten la ventana, de modo que
	// un clic durante un movimiento no cuenta dos veces.
	fn := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		now := args[0].Get("timeStamp").Float()
		if now-last < userActivityThrottleMs {
			return nil
		}
		last = now
		handler()
		return nil
	})

	// capture: scroll no burbujea, y en captura el documento ve cualquier
	// descendiente. passive: este handler nunca llama preventDefault, y
	// declararlo deja al navegador desplazar sin esperar a Go.
	opts := d.objectCtor.New()
	opts.Set("capture", true)
	opts.Set("passive", true)

	for _, ev := range userActivityEvents {
		d.document.Call("addEventListener", ev, fn, opts)
	}
}
```

Mechanics that are already decided — do not re-derive them:

- **`args[0].Get("timeStamp")`**, not `Date.now()`: the value is already on the
  event object, so it costs one property read instead of a global lookup plus a
  call.
- **`d.document`**, not `js.Global().Get("document")`: the singleton caches it
  (see the `domWasm` struct fields). `OnScrollCapture` predates that cache and
  does the global lookup — **leave `OnScrollCapture` exactly as it is**, this
  plan does not refactor it.
- **`d.objectCtor`** is the cached `Object` constructor already on the
  singleton; `element_wasm.go` → `scrollOptions` is the existing example of
  building an options object from it.
- `last` starts at `-userActivityThrottleMs` so the **first** event always
  passes: `timeStamp` starts near 0 on a freshly loaded page, and a `last` of 0
  would swallow every event in the first second.
- `userActivityEvents` is a package-level `var` because a slice cannot be a
  `const`. This does **not** violate `AGENTS.md` → "Internal State: Singleton
  Fields, Not Package Variables": that rule is about *mutable browser state*
  (`document`, `localStorage`, `lsUsedBytes`), which must live on the singleton
  so it is not shared across backend requests. This is a fixed, never-written
  lookup table. Do not move it onto `domWasm`.

### Repo rules that apply to this file

- **Zero standard library.** No `fmt`, `strings`, `errors`, `time`. Only
  `syscall/js` and `webtyp.com/fmt` if you need formatting — you do not here.
- **No `defer`/`recover`** — it silently does nothing in TinyGo WASM.
- **Slices, not maps** — the event set is a `[]string`, as written above.
- **No new exported types and no sub-packages.**

---

## Stage 3 — tests

### 3.1 Backend no-op — `dom_backend_test.go`

Append, modelled on the existing `TestOnScrollCaptureBackendIsANoop` directly
above it:

```go
// TestOnUserActivityBackendIsANoop guarantees that a component registering a
// presence listener in Init(ctx) still renders under SSR: the backend stub
// never invokes the handler.
func TestOnUserActivityBackendIsANoop(t *testing.T) {
	called := false
	OnUserActivity(func() {
		called = true
	})
	if called {
		t.Error("OnUserActivity backend stub should never invoke the handler")
	}
}
```

### 3.2 Browser test — `tests/uc_useractivity_test.go` (NEW FILE)

`//go:build wasm`, `package dom_test`. Per `AGENTS.md` → "Testing", all new
browser API tests live in `dom/tests/`, never in the package root, and mount
through `webtyp.com/dom/domtest` rather than hand-rolling a root.

`domtest.Fire(selector, eventType)` dispatches a **bubbling** `Event` at the
first match. A capture-phase listener on `document` sees it regardless of
bubbling, as long as the target is attached to the document — `domtest.Mount`
attaches it.

Three cases:

1. **It fires.** Mount a root, register `OnUserActivity` incrementing a counter,
   `domtest.Fire("#ua-root", "pointermove")`, assert the counter is `1`.
2. **It is throttled.** Fire `pointermove` twice back to back; assert the
   counter is still `1` — two synthetic events dispatched in the same tick carry
   near-identical `timeStamp`s, so the second falls inside the window.
3. **The window reopens, and a different event in the set also counts.** Sleep
   1100 ms, `domtest.Fire("#ua-root", "keydown")`, assert the counter is `2`.

> **Anti-footgun.** Case 3 needs `time.Sleep`, so this **test file** imports the
> standard `time` package. That is correct and intentional: the zero-stdlib rule
> in `AGENTS.md` protects the shipped WASM binary, and test files are not
> shipped. Do **not** "fix" that import to `webtyp.com/time`, and do not delete
> the case to avoid it.

Do not assert on the exact set of five event names from the test — the set is an
implementation detail. Cases 1 and 3 exercise two members of it, which is the
contract a consumer depends on.

---

## Stage 4 — docs

### 4.1 `docs/ARCHITECTURE.md`

Under `## 4. Events & Bindings` → `### Page-level listeners`, add a second
bullet after the `OnScrollCapture` one, in the same register (what it is, why it
cannot be an element listener, its limits, backend behavior):

- `OnUserActivity(handler func())` — registers the presence event set on
  `document` in capture phase; the handler is called when the user does anything
  anywhere in the page.
- State the three element-level failure modes from §0 as the reason it is
  document-level.
- State the ~1 call/second throttle and that the handler receives nothing.
- State that `dom` owns no timer and no notion of "idle".
- State "register once; there is no way to unregister". (Backend: no-op.)

### 4.2 `README.md`

The `## Events` table is for **element** builder methods — `OnUserActivity` is
not one, so it does not go in that table. Add a short `## Page-level listeners`
section immediately after `## Events`, listing `OnHashChange`,
`OnScrollCapture` and `OnUserActivity` with one line each, plus this example:

```go
// In Init(ctx) — once. dom reports the pulse; the timeout is yours.
dom.OnUserActivity(func() { shell.lastSeen = time.Now() })
```

VERIFY every claim against the code you just wrote. Do not document a parameter,
a default or a behavior that the implementation does not have.

---

## Acceptance criteria

1. `go build ./...` and `go vet ./...` green.
2. `GOOS=js GOARCH=wasm go build ./...` green.
3. `gotest` green — both lanes, including the new browser test.
4. `grep -rn "OnUserActivity" .` → exactly: the `DOM` interface entry, the
   `dom.go` delegator, the `dom_backend.go` stub, the `dom_frontend.go` method,
   the two tests, `README.md`, `docs/ARCHITECTURE.md`. **No other surface.**
5. `git diff --stat` lists **no new `.go` file** other than
   `tests/uc_useractivity_test.go`; no file gained a `syscall/js` import except
   `dom_frontend.go`, which already had one.
6. `git diff` does **not** touch `OnScrollCapture`, `OnHashChange`, or anything
   in `element.go` / `event.go`. This plan adds one listener and changes no
   existing behavior.
7. No file under `webtyp/layout` is modified — that is phase B, another repo.

| Stage | File | Action |
|---|---|---|
| 1 | `interface.dom.go` | `OnUserActivity` on the `DOM` interface |
| 1 | `dom.go` | package-level delegator + contract doc comment |
| 1 | `dom_backend.go` | no-op stub |
| 2 | `dom_frontend.go` | event set, throttle constant, `domWasm.OnUserActivity` |
| 3 | `dom_backend_test.go` | SSR no-op test |
| 3 | `tests/uc_useractivity_test.go` | **new** — fires, throttles, window reopens |
| 4 | `docs/ARCHITECTURE.md` | page-level listeners bullet |
| 4 | `README.md` | new "Page-level listeners" section |
