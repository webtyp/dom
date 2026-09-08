package dom

import (
	"webtyp.com/fmt"
)

var (
	shared    = &tinyDOM{}
	instance  = newDom(shared)
	idCounter uint64
)

// tinyDOM contains shared functionality between backend and WASM implementations.
type tinyDOM struct {
	log     func(v ...any)
	devMode bool
}

// generateID creates a unique ID for a component.
func generateID() string {
	idCounter++
	return fmt.Sprint(idCounter)
}

// ── one id, one node ────────────────────────────────────────────────────────
//
// Handlers and bindings in this framework resolve by id: an event is wired to
// `#3`, a signal patches `#3`. So two nodes sharing one id is not the ordinary
// HTML-validity nit — it is a component that renders, looks right, and does
// nothing, because the runtime resolved the other one. Nothing reports it.
//
// It happens one way: a Component instance stored in a field and rendered in
// two places. Its id is minted once and travels to both. The API-level fix is
// to make sharing unrepresentable — hand consumers a `func() Component`
// factory rather than a `Component` slot — and this is the net underneath.
//
// Deliberately plain globals, not a mutex: this package has no `sync` import
// by design, and idCounter above already assumes one renderer at a time. And a
// slice rather than a map — maps pull the map runtime into the TinyGo binary,
// which is the budget this whole ecosystem is built around. A render pass holds
// a few dozen ids; a linear scan over them is not the bottleneck.
var (
	idsInPass []fmt.KeyValue // Key is the id; Value carries the tag that claimed it
	passDepth int
)

// beginPass and endPass bracket ONE serialization of a tree. They nest — a
// child Component serializes from inside its parent's pass — and only the
// outermost bracket owns the set, so re-rendering the same tree (which
// legitimately re-emits the same ids) never trips the check.
func beginPass() {
	if passDepth == 0 {
		idsInPass = idsInPass[:0] // reuse the backing array across renders
	}
	passDepth++
}

func endPass() {
	passDepth--
	if passDepth <= 0 {
		passDepth = 0
		idsInPass = idsInPass[:0]
	}
}

// claimID records an id as written, and panics if this pass already wrote it.
// tag is the element that claimed it, so the message names both offenders.
func claimID(id, tag string) {
	if id == "" || passDepth == 0 {
		return
	}
	for _, seen := range idsInPass {
		if seen.Key == id {
			panic(fmt.Err("dom: id", id, "was written twice in one render, by <"+
				seen.Value+"> and <"+tag+"> — a single component instance is being",
				"rendered in two places, so one copy is inert: its handlers and",
				"bindings resolve to the other. Build one instance per place (a",
				"func() Component factory), or give each its own id"))
		}
	}
	idsInPass = append(idsInPass, fmt.KeyValue{Key: id, Value: tag})
}

// Render injects a component into a parent element.
func Render(parentID string, component Component) error {
	return instance.Render(parentID, component)
}

// Append injects a component AFTER the last child of the parent element.
func Append(parentID string, component Component) error {
	return instance.Append(parentID, component)
}

// Log provides logging functionality.
func Log(v ...any) {
	instance.Log(v...)
}

// Get retrieves an element by ID.
func Get(id string) (Reference, bool) {
	return instance.Get(id)
}

// GetByKey retrieves an element by its author key within a component's subtree.
func GetByKey(ownerID string, key string) (Reference, bool) {
	return instance.GetByKey(ownerID, key)
}

// OnHashChange registers a hash change listener.
func OnHashChange(handler func(hash string)) {
	instance.OnHashChange(handler)
}

// OnScrollCapture registra un listener de scroll en FASE DE CAPTURA sobre el
// documento, de modo que se dispara para CUALQUIER scroller de la página, no solo
// para la ventana.
//
// Existe porque el evento scroll no burbujea: se dispara únicamente en el elemento
// que se desplazó. Un shell que quiere reaccionar al scroll de su contenido no
// puede saber qué descendiente de qué componente es el que realmente desborda, y
// registrar el listener elemento por elemento lo obligaría a conocer el interior
// de otros paquetes.
//
// scrollTop es la posición vertical del elemento que disparó el evento. Con varios
// scrollers en pantalla los valores se intercalan: quien compare posiciones debe
// tolerarlo con un umbral, no asumir una serie continua.
//
// No hay forma de darlo de baja: es un listener del documento que vive lo que vive
// la página.
func OnScrollCapture(handler func(scrollTop float64)) {
	instance.OnScrollCapture(handler)
}

// GetHash gets the current hash.
func GetHash() string {
	return instance.GetHash()
}

// SetHash sets the current hash.
func SetHash(hash string) {
	instance.SetHash(hash)
}

// SetLog sets the logging function.
func SetLog(log func(v ...any)) {
	shared.log = log
}

// SetDevMode enables or disables development mode features.
func SetDevMode(on bool) {
	shared.devMode = on
}

// injectComponentID sets the component ID on the root element if not already set.
func injectComponentID(el *Element, id string) {
	if el.id == "" {
		el.id = id
	}
}

// Log provides logging functionality using the log function passed to New.
func (t *tinyDOM) Log(v ...any) {
	if t.log != nil {
		t.log(v...)
	}
}
