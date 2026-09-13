package dom

// DOM is the main entry point for interacting with the browser.
// It is designed to be injected into your components.
type DOM interface {
	// Render injecta un componente en un elemento padre.
	// 1. Llama a componente.Init(ctx) si existe (una sola vez)
	// 2. Llama a componente.Render() para obtener el árbol de elementos
	// 3. Inyecta el HTML resultante y enlaza bindings y eventos
	Render(parentID string, component Component) error

	// Append injecta un componente DESPUÉS del último hijo del elemento padre.
	// Útil para listas dinámicas.
	Append(parentID string, component Component) error

	// OnHashChange registra un listener para cambios en el hash de la URL.
	OnHashChange(handler func(hash string))

	// OnScrollCapture registra un listener de scroll en fase de captura sobre el
	// documento: se dispara para cualquier scroller de la página. Ver la función
	// de paquete del mismo nombre.
	OnScrollCapture(handler func(scrollTop float64))

	// OnUserActivity registra listeners de presencia en el documento en fase de
	// captura: el handler se llama cuando el usuario hace algo en cualquier
	// parte de la página. Ver la función de paquete del mismo nombre.
	OnUserActivity(handler func())

	// GetHash devuelve el hash actual de la URL (ej. "#help").
	GetHash() string

	// SetHash actualiza el hash de la URL.
	SetHash(hash string)

	// Get retrieves an element by ID.
	Get(id string) (Reference, bool)

	// Log provides logging functionality using the log function passed to New.
	Log(v ...any)
}

// Component is the minimal interface for components.
// All components must implement this for both SSR (backend) and WASM (frontend).
//
// NOTE: If your struct embeds Element, embed it as a VALUE, not a pointer:
//
//	type MyComponent struct {
//	  Element       // ✅ Correct — never nil
//	  // NOT: *Element // ❌ Wrong — nil pointer causes panic in renderToHTML
//	}
//
// This is because renderToHTML calls GetID() on every Component child before checking ViewRenderer.
type Component interface {
	GetID() string
	SetID(id string)
	String() string
	Children() []Component
}

// ViewRenderer returns a Node tree for declarative UI.
type ViewRenderer interface {
	Render() *Element
}

// elementNode identifies components that provide direct access to an underlying Element.
type elementNode interface {
	Component
	AsElement() *Element
}

// Ctx is handed to the Init hook. Register teardown for async resources (timers, websockets).
type Ctx interface {
	OnCleanup(fn func())
}

// initable is unexported but its method Init is exported.
// The engine asserts component.(initable) while the author only writes
// func Init(ctx dom.Ctx) and never sees the interface.
type initable interface {
	Init(Ctx)
}

// mountable is unexported but its method Mounted is exported.
// The engine asserts component.(mountable) while the author only writes
// func Mounted() and never sees the interface.
type mountable interface {
	Mounted()
}

// eventHandler represents a DOM event handler in the declarative builder.
type eventHandler struct {
	Name    string
	Handler func(Event)
}
