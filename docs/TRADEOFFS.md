# Trade-offs — reactividad de grano fino con signals tipados

Esta es la arquitectura elegida para `webtyp/dom`: el estado vive en **signals tipados**
(`SignalString`/`SignalBool`/`SignalNodes`, sin genéricos), y cada cambio parchea **solo** el nodo
del DOM ligado a ese dato — sin re-renderizar el componente, sin Virtual DOM.

Este documento enumera lo bueno y lo malo de esa decisión, sin maquillaje. El modelo y la API están
en [BINDING_MODEL.md](./BINDING_MODEL.md).

---

## Comparación con las alternativas

| | re-render completo | Virtual DOM | **signals (elegida)** |
|---|---|---|---|
| Trabajo por cambio | O(n) del subárbol | O(n) diff + asignaciones | **O(1) por binding** |
| Identidad del nodo (IME/foco/scroll) | se pierde | se preserva | **se preserva** |
| GC / tamaño en TinyGo | ok | malo (árbol sombra) | **bueno** |
| Construcción declarativa | sí | sí | **sí** |
| Camino quirúrgico explícito | no | no | **sí (`signal.Set`)** |
| "Magia" oculta | ninguna | diff | **auto-tracking (contenida)** |

---

## Pros

- **Actualizaciones quirúrgicas O(1).** `Set` toca solo el nodo ligado. No se recalcula ni se
  re-dibuja el resto del componente.
- **Se preserva la identidad del nodo.** El `<input>` nunca se reemplaza → el cursor no salta, la
  composición IME (acentos, CJK) no se rompe, el scroll y el foco se mantienen.
- **Imposible olvidar el "refresco".** No existe `Update()` manual (es interno). El footgun
  original — `Update()` dentro de `OnMount()` recursando hasta el stack overflow — deja de ser
  posible por construcción.
- **Binario pequeño y amable con el GC de TinyGo.** Sin árbol virtual, sin diff, sin asignaciones
  por frame.
- **Sin genéricos.** Tres tipos concretos (`SignalString`/`SignalBool`/`SignalNodes`) en vez de
  `Signal[T]`. Código más legible, alineado con la regla del ecosistema ("cero any, cero map") y
  con menos peso de compilación.
- **Auto-tracking: imposible olvidar una dependencia.** `BindTextFunc(fn)`/`DeriveString(fn)` se
  suscriben a los signals que la función *lee*. No hay lista de dependencias que mantener
  desactualizada.
- **`Render` puro → SSR isomórfico.** La misma función produce el HTML del servidor y la UI viva;
  el binding solo añade la suscripción en el cliente.
- **Contrato mínimo y forzado por el compilador.** `Render()` (+ `Init` opcional). `update`,
  `subscribable` e `initable` son internos: el autor no puede usarlos mal.

---

## Contras (costos reales) y cómo se mitigan

Cada contra con su mitigación en un solo lugar. La mayoría ya vive en [PLAN.md](./PLAN.md); la
columna "Dónde" lo indica. Las decisiones que no son un simple "está en el plan" se detallan debajo.

| # | Contra (costo real) | Mitigación | Dónde |
|---|---|---|---|
| 1 | **El estado debe vivir en un signal.** Guardarlo en un campo plano y mutarlo no actualiza la UI: fallo silencioso, sin error. | Builder tipado (`Text`/`Child`/`Attr`); se elimina `Add(...any)` → el dato dinámico solo se expresa con un binding que exige signal. | ✅ PLAN (Change 2) |
| 2 | **Auto-tracking es "magia" contenida.** `Get()` además suscribe; puede sorprender a quien espera un getter inerte. | Mecanismo documentado en BINDING_MODEL; `Peek()` (leer sin suscribir) evaluado y **omitido** por ahora. | ver abajo |
| 3 | **Más verboso que mutar un campo.** `s.Set(v)`/`s.Get()` en vez de `c.x = v`; ceremonia extra para estado trivial. | `Toggle()` (bool) y `Update(fn)` (leer-modificar-escribir) recortan los dos patrones más repetidos. | ✅ PLAN (Change 1) |
| 4 | **Una API por primitivo (sin genéricos).** Los números se formatean a `string`; más tipos exigirían `SignalInt`/`SignalFloat` a mano. | **Descartado:** no se añaden más tipos; `string`/`bool` cubren la frontera del DOM. | ver abajo |
| 5 | **Fugas si la limpieza falla.** Cada binding crea una suscripción y cada closure captura signals; un bug en la ruta de unmount retiene nodos muertos. | Ruta **única** de cleanup en el engine + test que afirma `subs` vacío tras desmontar. | ✅ PLAN (Change 4, test 9) |
| 6 | **Listas exigen claves correctas.** `BindChildren` reconcilia por `.Key(...)`; una clave mala (índice, duplicada) reusa el nodo equivocado. | `.Key()` por defecto al `id` + warning en `devMode` ante claves duplicadas/ausentes. | ✅ PLAN (Change 3) |
| 7 | **Semántica de re-montaje sutil.** `Render` corre por montaje; `Init` una vez. Saber qué se re-crea y qué persiste exige tener claro el contrato. | Diagrama de ciclo de vida + tabla del contrato + test de la invariante de `Init`. | ✅ PLAN (Change 6, test 8) |
| 8 | **Depurar el flujo reactivo es menos lineal.** "¿Quién disparó este parche?" se reconstruye signal→binding, no leyendo una llamada directa. | Trazado reactivo (`Set → patch #id`) bajo `devMode` — el mismo flag runtime que usa `app`. | ✅ PLAN (Change 4) |

### Sobre #1 — por qué API tipada en vez de un linter

Se descartó un analizador propio (`signalcheck`): es una herramienta aparte que mantener, y Go ya
trae `go vet`/`staticcheck`. La defensa correcta es **hacer el bug incompilable** con el sistema de
tipos: se elimina el `Add(...any)` genérico (el hueco donde un `"Hola, "+name.Get()` se cuela
pareciendo dinámico) y el contenido dinámico solo se expresa con un binding tipado (`BindText`, que
exige `*SignalString`). Detalle y justificación en PLAN.md (Change 2).

### Sobre #2 — decisión sobre `Peek()`: omitir por ahora

`Peek()` sería una lectura **sin** suscribir (leer el valor sin que el cálculo dependa de él), para
el caso raro de evitar re-ejecuciones no deseadas.

- **¿Lo usan frameworks reales?** Sí: Preact Signals (`signal.peek()`), SolidJS (`untrack()`),
  Angular Signals (`untracked()`), MobX (`untracked()`). Existe porque a gran escala aparece la
  necesidad de "leer sin reaccionar" (evitar bucles de realimentación, leer config/refs como
  contexto).
- **¿Es necesario ahora?** No. Es un escape hatch avanzado y poco frecuente; el modelo funciona sin
  él. Además es **puramente aditivo**: añadirlo más tarde no rompe nada.
- **Decisión:** **omitir** del plan inicial. Si aparece el caso real, se añade con la forma conocida
  (`Peek()`, como el `.peek()` de Preact). La "magia" del auto-tracking ya queda explicada en
  [BINDING_MODEL.md](./BINDING_MODEL.md), que es lo que de verdad quita la sorpresa.

### Sobre #4 — una API por primitivo: descartado

No se añadirán `SignalInt`/`SignalFloat` ni un generador. Los números se formatean a `string` en el
componente y se usa `SignalString`. La frontera del DOM es `string`/`bool`; más tipos de signal (o
codegen para mantenerlos) son complejidad innecesaria cuando el objetivo es simplificar.

---

## Cuándo el costo vale la pena

- **Sí:** UI interactiva con estado que cambia (formularios, toggles, listas, navegación) — que es
  el 100% de los componentes de este ecosistema.
- **Marginal:** contenido totalmente estático sin estado. Ahí un componente solo escribe `Render()`
  y nunca crea un signal; el costo es cero porque no usas nada de la maquinaria reactiva.

La conclusión del diseño: los contras son sobre todo **disciplina y curva de aprendizaje**, no
límites de rendimiento; los pros eliminan una clase entera de bugs (refrescos olvidados, foco/IME
rotos) que ya golpearon al proyecto.
