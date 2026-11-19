---
PLAN: "feat: build DOM nodes without innerHTML and add StyleProp for strict CSP"
EXECUTOR: jules
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit, push and open the pull request. Never ask whether to proceed or finalize — nobody answers, and a session that waits ends without your work.

# Plan: `dom` sin `innerHTML` y con `StyleProp` (CSP estricta + Trusted Types)

Master: `SECURITY_OBSERVABILITY_MASTER_PLAN.md` (D3, O3b-1).

## Tools

```bash
export GOPRIVATE=webtyp.com/*,github.com/veltylabs/*
export GOFLAGS=-mod=mod
go install webtyp.com/devflow/cmd/gotest@latest
export PATH="$PATH:$(go env GOPATH)/bin"
```

## Ecosystem search

- Buscado en `~/Dev/Project/webtyp` (`dom`, `html`, `components`, `layout`, `css`).
- Ningún paquete del ecosistema ofrece API CSSOM (`element.style.setProperty`) ni montaje de nodos DOM sin `innerHTML`/`insertAdjacentHTML`.
- **Decisión:** extender `webtyp.com/dom` (dueño único del montaje DOM en WASM y del tipo `*dom.Element`).

## Design gate

1. **Prior art:**
   - [MDN — CSP `style-src`](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Content-Security-Policy/style-src): los atributos HTML `style="..."` quedan bloqueados sin `'unsafe-inline'`, pero `element.style.setProperty(...)` vía CSSOM sí está permitido.
   - [MDN — Trusted Types API](https://developer.mozilla.org/en-US/docs/Web/API/Trusted_Types_API): `require-trusted-types-for 'script'` bloquea `innerHTML` e `insertAdjacentHTML` salvo `TrustedHTML`; construir nodos con `createElement`/`createElementNS`/`setAttribute`/`appendChild` evita sinks HTML por completo.
   - [React DOM](https://react.dev/reference/react-dom/components/common) / [SolidJS](https://docs.solidjs.com/): montan nodos DOM directamente y aplican propiedades de estilo vía `element.style.setProperty`.
2. **Novice-name test:** `Element.StyleProp(property, value string) *Element` — verbo/sustantivo literal ("propiedad de estilo"), distinto de `Attr("style", ...)` (atributo HTML bloqueado por CSP).
3. **Complexity ledger:**
   - Conceptos a aprender: `+1` (`StyleProp`) / `-0`
   - Archivos a tocar para estilizar valores dinámicos: `0`
   - Líneas en el call site: `0` (reemplaza `Attr("style", ...)`)
   - Formas de hacer lo mismo: `0` (`Attr("style", ...)` deja de usarse en el ecosistema; `StyleProp` es la única vía CSSOM).
4. **Where it belongs:** `webtyp.com/dom` es la única capa que construye `*Element` y toca `syscall/js` en WASM. Mantener esto en `dom` evita acoplar `components` o `layout` a `syscall/js` o a Trusted Types.
5. **What does this change delete?** Elimina los 3 usos directos de `innerHTML` / `insertAdjacentHTML` en `dom/dom_frontend.go` (`:202`, `:279`, `:1028`) para árboles `*Element`.

## APIs used

| Symbol | Exact signature | Tag | File |
|---|---|---|---|
| `fmt.KeyValue` | `type KeyValue struct { Key, Value string }` | `v1.0.2` | `webtyp.com/fmt` |

If a signature in the published tag differs from this table, stop and name the symbol in the PR; never adapt it by guessing.

## Reglas de entrega y anti-footguns

- Este paquete compila a WASM con TinyGo: **no usar `map` ni stdlib `strings`/`fmt`/`time`** en código compilable a WASM.
- No editar nunca `go.mod`/`go.sum` a mano; usar `go mod tidy`.
- Hacer commit después de cada etapa. No usar `git reset`, `git restore`, `git checkout -- <file>` ni `git stash`.

---

## Etapa 1: Diagrama y pruebas (TDD)

1. Crear `docs/diagrams/DOM_MOUNT.md` (cabecera de contexto, `flowchart TD`, walkthrough numerado) describiendo el montaje sin `innerHTML`:
   - Camino `*Element` → `createElement` / `createElementNS("http://www.w3.org/2000/svg", ...)` → `setAttribute` (sin atributo `style`) → `style.setProperty(prop, val)` para `styles` → `appendChild` de hijos y nodos texto (`createTextNode`).
   - Camino fallback (`Component.String()` sin `*Element`) → política única de Trusted Types (`webtyp-dom`) si `window.trustedTypes` existe, si no cadena directa (entornos sin Trusted Types).
2. Agregar pruebas en `tests/style_prop_test.go` (y registrarlas en `docs/TESTS.md` con formato `| [TestName](../tests/style_prop_test.go) | regla |` bajo `| Test | Rule |`):
   - `TestStyleProp_NotEmittedAsHTMLAttr`: `NewElement("div").StyleProp("--meter-fill", "40%").String()` NO contiene `style=` en el HTML serializado (SSR/HTML limpio bajo `style-src 'self'`).
   - `TestStyleProp_LastValueWins`: llamar `StyleProp("grid-row", "1").StyleProp("grid-row", "2")` sobrescribe la propiedad sin duplicarla.
   - `TestStyleProp_IgnoresEmptyProperty`: `StyleProp("", "10px")` es no-op.

## Etapa 2: `Element.StyleProp` en `element.go`

En `element.go`:
1. Agregar campo `styles []fmt.KeyValue` a `type Element struct`.
2. Agregar método exportado:

```go
// StyleProp sets a single CSSOM style property (applied via element.style.setProperty
// in the browser, never emitted as an inline HTML style="..." attribute, so it works
// under strict Content-Security-Policy style-src 'self').
func (b *Element) StyleProp(property, value string) *Element {
	if property == "" {
		return b
	}
	for i, s := range b.styles {
		if s.Key == property {
			b.styles[i].Value = value
			return b
		}
	}
	b.styles = append(b.styles, fmt.KeyValue{Key: property, Value: value})
	return b
}
```

3. Asegurar que `renderElement` / `String()` NO serializa `b.styles` como atributo HTML `style="..."`.

## Etapa 3: Montaje por nodos DOM en `dom_frontend.go`

En `dom_frontend.go`:
1. Implementar `buildNode(el *Element, children *[]Component, ownerID string, svgNS bool) js.Value`:
   - Si `el.tag == "svg"` o `svgNS == true`: crear con `d.document.Call("createElementNS", "http://www.w3.org/2000/svg", el.tag)` y propagar `svgNS = true` a los hijos.
   - En otro caso: `d.document.Call("createElement", el.tag)`.
   - Aplicar `id` (asegurando ID si tiene eventos/bindings), `class` (unir `el.classes` con `" "`), `data-key` si `el.key != ""`, `autofocus`, y todos los `el.attrs` vía `node.Call("setAttribute", attr.Key, attr.Value)`.
   - Aplicar `el.styles` vía `styleObj := node.Get("style"); styleObj.Call("setProperty", s.Key, s.Value)`.
   - Registrar eventos y bindings igual que hace `renderToHTML`.
   - Recorrer `el.children`:
     - `string`: `node.Call("appendChild", d.document.Call("createTextNode", s))`
     - `*Element`: recursión con `buildNode`
     - `Component` / `ViewRenderer` / `elementNode`: inicializar, rastrear en `*children`, construir y adjuntar.
2. En `Render` (`dom_frontend.go:202`):
   - Limpiar hijos de `parent` con `parent.Call("replaceChildren")` (en vez de `parent.Set("innerHTML", ...)`).
   - Si hay `renderedRoot != nil`: adjuntar `d.buildNode(renderedRoot, &children, component.GetID(), false)` con `parent.Call("appendChild", node)`.
   - Solo si el componente cayó en `component.String()` (legacy sin `*Element`): asignar vía `d.setTrustedHTML(parent, html)` usando una única política `trustedTypes.createPolicy("webtyp-dom", ...)` (cacheada en `d`).
3. En `Append` (`dom_frontend.go:279`):
   - Si hay `appendRoot != nil`: `parent.Call("appendChild", d.buildNode(appendRoot, &children, component.GetID(), false))` (sin `insertAdjacentHTML`).
   - Fallback legacy: crear `<template>` con `d.setTrustedHTML` y adjuntar su `content`.
4. En la reconciliación por clave (`dom_frontend.go:1026-1035`):
   - Reemplazar `tempDiv.Set("innerHTML", html)` por `newNode := d.buildNode(n, &comps, parentID, false)` y luego `insertBefore`/`appendChild`.
5. En `domtest/domtest.go:28`: reemplazar `existing.Set("innerHTML", "")` por `existing.Call("replaceChildren")`.

## Etapa 4: Verificación y documentación

1. Verificar con grep que no quedan asignaciones directas a `innerHTML` ni `insertAdjacentHTML` fuera del helper `setTrustedHTML`:
   `grep -rn 'innerHTML\|insertAdjacentHTML' dom_frontend.go domtest/`
2. Actualizar `README.md` (≤ 150 líneas, 4 secciones obligatorias: `Why This Exists`, `Before You Start`, `Step-by-Step Guide`, `Documentation Map`), `docs/ARCHITECTURE.md` (4 secciones) y `docs/TESTS.md`.
3. Ejecutar delivery check:
   ```bash
   gotest
   ```

## Tabla de etapas

| Etapa | Archivos | Qué |
|---|---|---|
| 1 | `docs/diagrams/DOM_MOUNT.md`, `tests/style_prop_test.go`, `docs/TESTS.md` | Diagrama y pruebas TDD |
| 2 | `element.go` | `Element.StyleProp(property, value)` sin emitir `style="..."` |
| 3 | `dom_frontend.go`, `domtest/domtest.go` | Montaje directo de nodos DOM + política única Trusted Types para fallback |
| 4 | `README.md`, `docs/ARCHITECTURE.md`, `docs/TESTS.md` | Documentación y `gotest` verde |
