---
PLAN: "fix(dom): signal tracker compares by concrete pointer — no reflection in the wasm binary"
EXECUTOR: jules
REVIEWER: none
---

# Plan — `tracker.add` sin `==` entre interfaces

> Master: `webtyp/docs/NO_REFLECTION_MASTER_PLAN.md` (ola 1). Sin API pública nueva.

## 1. El problema

`signal.go:171-178`:

```go
func (t *tracker) add(s subscribable) {
	for _, sig := range t.signals {
		if sig == s {   // == entre dos interfaces
			return
		}
	}
	t.signals = append(t.signals, s)
}
```

En TinyGo, `==` entre interfaces compila a `runtime.interfaceEqual`, que llama a
`reflectValueEqual(reflectlite.ValueOf(x), reflectlite.ValueOf(y))`. Esto mete
`internal/reflectlite` (~7 KB) en **todo** binario que use señales, porque `Get()` de cada señal
llama a `tracker.add`. Medido: quitando este `==`, reflectlite desaparece del login de mjosefa-cms y
el binario baja 11,5 KB raw.

## 2. La corrección

Las tres señales (`SignalString`, `SignalBool`, `SignalNodes`) repiten los mismos campos `subs`,
`nextID` y el mismo cuerpo de `subscribe`. Extraer esa parte en una celda común y comparar **punteros
a la celda** (tipo concreto: comparación de punteros normal, sin reflexión).

1. En `signal.go`:

   ```go
   // cell is what every signal shares: its subscribers. The tracker compares
   // signals by *cell — a concrete pointer — because == between interface
   // values compiles, under TinyGo, to runtime.interfaceEqual, which pulls
   // internal/reflectlite into every binary that reads a signal.
   type cell struct {
   	subs   []sub
   	nextID uint64
   }

   func (c *cell) subscribe(fn func()) (unsub func()) {
   	c.nextID++
   	id := c.nextID
   	c.subs = append(c.subs, sub{id: id, fn: fn})
   	return func() { c.subs = removeSub(c.subs, id) }
   }
   ```

2. Cada señal reemplaza sus campos `subs` y `nextID` por `cell` embebida (por valor). Todo uso de
   `s.subs` sigue compilando por promoción (`notify(s.subs)`).
3. El `subscribable` y los métodos por tipo:

   ```go
   type subscribable interface {
   	subscribe(fn func()) (unsub func())
   	signalCell() *cell
   }
   ```

   En cada tipo (ejemplo `SignalString`; igual en `SignalBool` y `SignalNodes`), conservando la
   protección contra receptor nil que existe hoy:

   ```go
   func (s *SignalString) subscribe(fn func()) (unsub func()) {
   	if s == nil {
   		return func() {}
   	}
   	return s.cell.subscribe(fn)
   }

   func (s *SignalString) signalCell() *cell {
   	if s == nil {
   		return nil
   	}
   	return &s.cell
   }
   ```

4. `tracker.add`:

   ```go
   func (t *tracker) add(s subscribable) {
   	c := s.signalCell()
   	for _, sig := range t.signals {
   		if sig.signalCell() == c {
   			return
   		}
   	}
   	t.signals = append(t.signals, s)
   }
   ```

5. Buscar en el paquete cualquier otro `==`/`!=`/`switch` entre valores de interfaz con operandos no
   nil y aplicar lo mismo. Comando de verificación al final.

## 3. Tests (rojo primero donde aplica)

- `tests/` ya cubre el comportamiento de las señales y `Derive*`: debe seguir verde sin cambios.
- Agregar en `tests/` un caso: un `DeriveString` que lee **la misma** señal dos veces se recalcula
  una sola vez cuando esa señal cambia (la deduplicación del tracker sigue funcionando).
- Guardia de tamaño: si `tinygo` está disponible (si no, `t.Skip`), compilar un `main` mínimo que crea
  un `NewString` y llama a `Get()` dentro de un `DeriveString`, con
  `tinygo build -target wasm -opt=z -panic=trap -size=full`, y fallar si la salida contiene
  `internal/reflectlite`. Este test es rojo hoy.

## 4. Criterios de aceptación

- `grep -n 'sig == s' signal.go` → vacío.
- `gotest` verde (vet, race, tests, wasm).
- No hay símbolos exportados nuevos: `git diff | grep '^+func [A-Z]'` → vacío.
- `tinygo build -target wasm -opt=z -panic=trap -size=full` del test de tamaño: sin `internal/reflectlite`.

## 5. Restricciones

Las de `AGENTS.md` (sin `map` — usar `[]fmt.KeyValue` —, tests en `tests/`), más las de este plan:
nada de `reflect`, nada de `unsafe`, y ningún `==`/`!=`/`switch` entre valores de interfaz con
operandos no nil.
