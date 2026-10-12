---
PLAN: "feat(dom): read and clear the URL query, and navigate with a POST form (payment gateway hand-off)"
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 13306400392430250073
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

> **Phase A2** of
> [`veltylabs/mjosefa-cms/docs/RESERVA_Y_PAGOS_MASTER_PLAN.md`](https://github.com/veltylabs/mjosefa-cms/blob/main/docs/RESERVA_Y_PAGOS_MASTER_PLAN.md).
> Gate for Phase P's `ui/` (`veltylabs/payment_manager`) and Phase B
> (`veltylabs/appointment_booking`).

# Plan — `dom`: `GetQuery`, `ClearQuery`, `NavigatePost`

## 0. Context (read first)

A web payment gateway (Transbank Webpay Plus) works like this:

1. The server creates a transaction and gets back a gateway URL and a token.
2. **The browser must leave the app with an HTML form POST** to that URL carrying the token
   as a form field (`token_ws`). The gateway does not accept the token in a JS fetch.
3. After paying, the gateway sends the browser back to the app's server, which redirects to
   the app page with the result in the query string: `/?payment=<id>&status=PAID#appointment_booking`.
4. The app must read `payment` and `status` from `location.search`, act, and then remove them
   so a reload does not repeat the action.

Today `dom` exposes only the hash (`GetHash`, `SetHash`, `OnHashChange`). The shell
(`layout/platformd`) routes by the whole hash, so the result cannot travel in the hash.
Components and module UIs must not import `syscall/js`; reaching the browser is `dom`'s job.

## Design gate

### 1. Prior art
- Browser APIs: `URLSearchParams.get`, `history.replaceState`, and the "auto-submitting
  hidden form" used by every hosted payment page integration (Transbank, Adyen HPP,
  PayPal Standard, 3-D Secure ACS redirects).
- Go: `url.Values.Get` returns `""` for absent and present-but-empty alike; we return
  `(value, ok)` as `router.QueryParam` already does in this ecosystem.

### 2. Novice-name test
- `GetQuery(key) (string, bool)` — "get this query-string parameter of the page URL". Pairs
  with the existing `GetHash`.
- `ClearQuery()` — "remove the query string from the address bar without reloading".
- `NavigatePost(url, fields)` — "leave the page by POSTing these fields to url".

### 3. Complexity ledger
+3 package functions, +3 methods on the internal `instance` interface (`interface.dom.go`),
+3 wasm implementations, +3 backend no-ops. No type added; `fields` reuses `fmt.KeyValue`.

### 4. Where it belongs
`dom` is the only package allowed to touch `window`/`document` (`dom_frontend.go`,
`//go:build wasm`); its backend twin (`dom_backend.go`, `!wasm`) keeps SSR and tests compiling.

### 5. What it deletes
Nothing today. It prevents the first module from importing `syscall/js` to do a gateway
hand-off.

## 1. Rules (inline)

- `dom_frontend.go` is `//go:build wasm` and may use `syscall/js`; nothing else in the
  package may. `dom_backend.go` is `//go:build !wasm`.
- No stdlib `strings`/`strconv`/`errors`/`fmt` in non-test code: use `webtyp.com/fmt`.
  No `map`.
- **All tests in `tests/`** (external package, exported API only); wasm tests carry
  `//go:build wasm`. Run with `gotest` (never `go test`).

## 1.1 Method: diagram → failing tests → code (TDD), 100% coverage of new code

- Prerequisite: `go install webtyp.com/devflow/cmd/gotest@latest`.
- Source of truth: [`docs/diagrams/location.md`](diagrams/location.md) (already written). Every
  branch has its exact test in §4.
- Order: stage 1 (signatures, `panic("not implemented")` in both implementations) → write all
  tests of §4 → `gotest` → **confirm they fail** → commit `test(dom): failing location tests`
  → stage 2 → green → commit `feat(dom): …`.
- Coverage: the new non-wasm code (`GetQuery`, `ClearQuery`, `NavigatePost` in `dom.go` and
  their `dom_backend.go` no-ops) at 100.0% in `go tool cover -func` of the non-wasm run. The
  wasm implementations are covered branch by branch by the wasm tests of §4 (coverage is not
  measured in wasm).

## 2. Stage 1 — API

`dom.go`, next to `GetHash`/`SetHash` (same style, same doc density):

```go
// GetQuery returns the value of key in the page URL's query string
// (window.location.search), percent-decoded. ok is false when key is absent;
// a present but empty parameter ("?k=") returns "", true.
func GetQuery(key string) (string, bool) { return instance.GetQuery(key) }

// ClearQuery removes the query string from the address bar without reloading
// or adding a history entry. The path and the hash are kept.
func ClearQuery() { instance.ClearQuery() }

// NavigatePost leaves the page by submitting a hidden HTML form with method
// POST to url, with one hidden input per field (Key = name, Value = value).
// It is how a page hands the browser over to a hosted payment page. Nothing
// after it runs meaningfully: the page is unloading.
func NavigatePost(url string, fields []fmt.KeyValue) { instance.NavigatePost(url, fields) }
```

`interface.dom.go`: add the three methods to the interface, after `SetHash`, with one-line
comments pointing to the package-level functions.

## 3. Stage 2 — implementations

`dom_frontend.go` (wasm):

- `GetQuery`: `params := js.Global().Get("URLSearchParams").New(js.Global().Get("location").Get("search"))`;
  `if !params.Call("has", key).Bool() { return "", false }`; return `params.Call("get", key).String(), true`.
- `ClearQuery`: `loc := js.Global().Get("location")`; build
  `loc.pathname + loc.hash` and call `js.Global().Get("history").Call("replaceState", js.Null(), "", that)`.
- `NavigatePost`: create `form` (`method="post"`, `action=url`, `style.display="none"`), one
  `input type=hidden` per field (`name=Key`, `value=Value`), append the form to
  `document.body`, then `form.Call("submit")`.

`dom_backend.go` (`!wasm`): `GetQuery` returns `"", false`; `ClearQuery` and `NavigatePost`
do nothing.

## 4. Tests — diagram → test traceability (all in `tests/`)

| Branch | Test | File |
|---|---|---|
| `Q1-!wasm`, `C1-!wasm`, `N1-!wasm` | `TestBackend_LocationHelpersAreNoops` (`GetQuery` → `"", false`; the other two do not panic) | `backend_location_test.go` (`!wasm`) |
| `Q2-no` | `TestWASM_GetQuery_Missing` | `uc_location_query_test.go` (`wasm`) |
| `Q2-yes, empty` | `TestWASM_GetQuery_PresentButEmpty` | same |
| `Q2-yes, value` | `TestWASM_GetQuery_DecodesValue` (`?q=a%20b` → `"a b"`) | same |
| `C1-wasm` | `TestWASM_ClearQuery_KeepsPathAndHash` (and `history.length` unchanged) | same |
| `N2-some` | `TestWASM_NavigatePost_BuildsHiddenFormAndSubmits` | same |
| `N2-none` | `TestWASM_NavigatePost_NoFields` | same |

Test mechanics (wasm only, inside the test file, never in the package):
- Set the URL with `history.replaceState(null, "", "/?payment=p1&status=PAID&empty=#home")`
  through `syscall/js`.
- `NavigatePost`: a real submit would unload the runner. Save
  `HTMLFormElement.prototype.submit`, replace it with a JS function that copies `this.action`,
  `this.method` and each input's `name`/`value` into a global object, call `NavigatePost`,
  restore the original, then assert method `post`, the action URL and the inputs.

## 5. Stage 3 — docs

`README.md` (and `docs/ARCHITECTURE.md` if it lists the browser-facing functions): add the
three functions next to `GetHash`/`SetHash`.

## 6. Acceptance criteria

- `gotest` green (vet, race, tests, wasm).
- Every test of §4 exists with its exact name in `tests/`.
- The coverage criterion of §1.1 holds.
- The PR history shows the red `test(dom)` commit before the green `feat(dom)` one.
- `grep -rn "syscall/js" --include=*.go . | grep -v _test.go | grep -v dom_frontend.go` lists
  nothing new compared to `main`.
- Before opening the PR: `git diff --stat main` lists every file of the stages table, and no
  file is deleted that this plan does not order deleted.

## 7. Stages table

| Stage | Files |
|---|---|
| 1 | `dom.go`, `interface.dom.go` |
| 2 | `dom_frontend.go`, `dom_backend.go` |
| 1–2 | `tests/uc_location_query_test.go`, `tests/backend_location_test.go` (written red first) |
| 3 | `README.md`, `docs/ARCHITECTURE.md` |
