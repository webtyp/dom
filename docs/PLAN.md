---
PLAN: "fix(dom): dom is the ONLY source of element ids — components may not create their own"
TAG: v0.13.11
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 2484467419766286466
---

# PLAN — `dom` única fuente de ids: los componentes no crean ids propios

Orquestador: `webtyp/docs/DEMO_AGENDA_MASTER_PLAN.md`. Reportado desde la Etapa
D: `app-demo` monta DOS `calendarslider` en un mismo render (filtro de
`reservation` + el de `agenda`/`ScheduleEditor`), y ambos emiten ids globales
hardcodeados `cs-m-2026-09` / `cs-d-…`. `dom.claimID` panica:

```
dom: id cs-m-2026-09 was written twice in one render, by <div> and <div>
```

## Decisión de arquitectura (cerrada con el dueño del framework — no negociable)

**`dom` es la ÚNICA fuente de ids.** Un componente NO crea ids globales propios
(no `Element.ID("cs-m-…")`). `dom` genera los ids de los elementos, por
instancia de componente, al serializar. Esto elimina toda la clase de error:
dos instancias del mismo componente en el mismo render no colisionan JAMÁS
porque nunca compiten por un id que escribió el autor.

La analogía: `Key` es la identidad del autor (`BindChildren` reconcile,
`data-` para tests); `id` es un detalle del DOM que solo `dom` conoce. Un
componente no tiene por qué saber — ni poder — fijar el `id` que el runtime
usará.

## Qué cambia en `dom`

1. **`Element.ID(id)` deja de fijar el id del nodo cuando el elemento vive
   DENTRO del Render de un componente** (bajo un `ownerID` ≠ ""): el id que el
   autor escribe es **descartado/en-árbol** y `dom` asigna el suyo. Esto es un
   cambio de contrato: el método existe para los elementos que el chasis/
   `<html>`/la app declara (raíces, integración con CSS externo), no para los
   hijos de un componente.
   - Concretamente: en `serializeElement` (backend) y `renderToHTML` (wasm)
     explode: si `passHasOwner` (bajo un componente con id de instancia),
     un `el.id` puesto por el autor NO se escribe ni se claima; en su lugar se
     asigna `generateID()` (o el id de inyección del owner si es la raíz del
     componente). Así los ids escritos en un render vienen todos de
     `generateID()` → únicos por construcción → `claimID` nunca entra en el
     camino de dos instancias hermanas.
   - Un `el.id` explícito SOLO se respeta a nivel de raíz (sin owner): p. ej.
     `id="app"`, `id="body"`. Es la superficie de integración, documentada.
2. **`claimID` sigue existiendo** como red de seguridad ABSOLUTA (si dos ids
   iguales aparecen igual, pánico) — pero con la regla de arriba no debería
   poder dispararse por duplicación entre instancias. Se conserva para el caso
   de "una instancia renderizada en dos lugares" (mismo id de instancia re-inyectado
   — seguirá paniqueando, mensaje igual).
3. **`Key()` se convierte en el contrato para identidad dentro del autor.**
   `BindChildren` ya reconcilia por `Key`. Un componente que hoy usa `ID()` para
   "referenciar internamente un nodo" (p. ej. `slideToMonth` con
   `Get("cs-m-"+key)`) pasa a usar `Key` + un lookup por clave DENTRO de su
   subárbol — o directamente un signal/estado, no un `Get` por id. En la Etapa
   I / DOM del plan se auditan los componentes (`calendarslider`, `usermenu`,
   `modaldialog`, `platformd`) y se reescribe el uso de `Get(id)`/`ID()` interno:
   - busquemos por `Key` (el autor lo conoce) →
     `dom.GetByKey(ownerID, key)` o un `Element.Scan(key)` — API a diseñar en
     este plan (ver "Piezas").
4. **`dom.Get(id)`** sigue para interés del framework/chasis (botón de rutas,
   `#app`), y para tests se usa `data-`/selector, no `id`. Dentro de un
   componente ya NO se llama `Get(idAutoral)` (no existe tal id).

## Piezas nuevas (si hacen falta)

Diseñar el reemplazo de "referencia interna por id" que `calendarslider` y
otros necesitan. Opciones candidatas (elegir durante la implementación la que
sea más pequeña, NO inventar superficie de más):

- `Element.Key("…")` ya existe. Si un componente necesita saltar a un nodo de
  su propio subárbol por clave: dar un medio de `dom` — sugerencia:
  `GetByKey(ownerComponent, key) Reference` (búsqueda por `el.key` bajo el
  subárbol del owner) o simplemente recomendar que la lógica viva en el
  componente (no conozca el DOM): p. ej. `calendarslider` guarda el mes actual
  en una señal y los botones setean esa señal + `ScrollIntoView` sobre un nodo
  que el propio Render puede exponer vía `Key`. Escoger lo mínimo.

## Tests

- `dom/serialize_test.go` (backend):
  - **`TestComponentChildId_IsDomGenerated`**: un componente cuyo Render hace
    `Div().ID("foo")` bajo un owner → el HTML NO contiene `id="foo"` y SÍ un id
    generado. Dos instancias → ids distintos en ambos, cero panic.
  - **`TestRootId_StillHonored`**: `Div().ID("app")` sin owner → se respeta
    `id="app"` (integración).
  - **`TestSameInstanceTwice_StillPanics`**: regresión — la misma instancia en
    dos lugares sigue paniqueando (`claimID`).
- `dom/tests/uc_instance_ids_test.go` (`//go:build wasm`): montar dos
  componentes hermanos con `ID("cell")` interno → ambos viven, DOM desambiguado,
  click funciona en ambos, el handler de A no pued? tocar el nodo de B.
- `gotest ./...` verde.

## Criterios de aceptación

- Un `Element.ID(...)` dentro del Render de un componente NO produce un id
  global: el estado "dos instancias en el mismo render" ya no puede darse.
- El corner de `calendarslider` (dos en la demo) carga sin panic, y cada uno
  navega por sus propios meses (referencia interna reescrita a `Key`/señal).
- `README.md`/`docs/ARCHITECTURE.md` de `dom` documentan la regla: **"el id de
  un elemento hijo de un componente lo genera dom; el autor usa Key/`data-`"**.
- `AGENTS.md` de `dom` agrega la regla a la sección "Component Contract" (o una
  sección nueva "Element ids are owned by dom").

## Fuera de alcance

- Cambiar ids en `calendarslider` — ese parche queda descartado (contradice la
  decisión: el componente no toca ids). El plan `components` borrado lo
  confirmaba.
- `Get(id)` para autor dentro de un componente: se elimina el patrón, no se
  inventan aliases de más.

## Nota de ejecución

Este plan es de **`webtyp/dom`** (raíz de composición del arnés). NO tocar
componentes en este repo; la migración de `calendarslider`/`usermenu`/etc. a
`Key`/señales va en un plan posterior (Etapa I del master) UNA VEZ que `dom`
imponga la regla. Aquí solo: hacer que `dom` sea la fuente única y agregar la
documentación de la regla.