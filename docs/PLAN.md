---
PLAN: "feat: Navigate(url) leaves the page with a plain GET"
TAG: v0.13.27
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 14831625684299623375
---

> Read AGENTS.md at the repo root before any change.

# Plan — `dom.Navigate(url)`

## Why

`dom.NavigatePost` hands the browser to a hosted payment page by POST (Webpay needs a form field).
Other processors (TUU's online payment) give a URL that the browser must simply open with a GET. Today
a module would have to touch `js` directly to do that, which the ecosystem forbids. `Navigate` is the
GET sibling of `NavigatePost`.

## Delivery rules

- Never pause to ask permission or to report progress. When the Delivery check passes: commit, push,
  open the PR. Stuck for 30 minutes on one point: open the PR anyway, listing what fails.
- A PR that does not compile, or whose tests could not run, is a failed delivery.

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
```

## Ecosystem search

`grep -rn "location\|Navigate" --include=*.go` over `webtyp/*`: only `dom.NavigatePost`, `GetHash`/`SetHash`,
`ClearQuery`. **Extend** `dom`.

## Design gate

1. **Prior art.** `window.location.assign(url)` (DOM), `router.push` (Vue/Next) for in-app, `http.Redirect`
   (server). This is a full-page exit to another origin, the browser's `assign`.
2. **Novice name.** `Navigate(url)` — "navigate to url"; pairs with the existing `NavigatePost`.
3. **Ledger.** +1 function; ways to leave the page by GET: 0 → 1.
4. **Where.** `dom` owns every browser global.
5. **Deletes.** Nothing (new capability).

## Stage 1 — Code

- `interface.dom.go`: add `Navigate(url string)` next to `NavigatePost` (line 44).
- `dom.go`, right after `NavigatePost`:

```go
// Navigate leaves the page and opens url with a plain GET, as a click on a link would. It is how a
// page hands the browser over to a hosted payment page that needs no form fields. Nothing after it
// runs meaningfully: the page is unloading.
func Navigate(url string) { instance.Navigate(url) }
```

- `dom_backend.go`: `func (d *domBackend) Navigate(url string) { _ = url }` (no-op, like `NavigatePost`).
- `dom_frontend.go`, after `NavigatePost`: create an `<a>`, set `href` to url, hide it
  (`style.display = "none"`), append it to `document.body`, call `click()`. (A link, not
  `location.assign`, so the browser test can intercept it the same way `NavigatePost` intercepts `submit`.)

## Stage 2 — Test (`tests/uc_location_query_test.go`, build tag `wasm`)

Copy `setupNavigatePostMock` as `setupNavigateMock`: replace `HTMLAnchorElement.prototype.click` with a
function that records `this.href` into `__mockNavigateData.href`; restore it in the teardown.

`TestWASM_Navigate_ClicksLink`: `dom.Navigate("https://pay.example.com/checkout?id=7")` → recorded href is
exactly `https://pay.example.com/checkout?id=7`.

## Stage 3 — Docs

`README.md` line 189 (next to `NavigatePost`): `- dom.Navigate(url): Leave the page by opening url with a
plain GET (payment pages that need no form fields).`

## Delivery check

```bash
gotest
```

## Stages

| # | What | Files |
|---|---|---|
| 1 | `Navigate` | `interface.dom.go`, `dom.go`, `dom_backend.go`, `dom_frontend.go` |
| 2 | Browser test | `tests/uc_location_query_test.go` |
| 3 | Docs | `README.md` |
