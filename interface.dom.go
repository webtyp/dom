package dom

// DOM is the main entry point for interacting with the browser.
// It is designed to be injected into your components.
type DOM interface {
	// Render injects a component into a parent element.
	// 1. Calls component.Init(ctx) if present (exactly once)
	// 2. Calls component.Render() to obtain the element tree
	// 3. Injects the resulting HTML and wires bindings and events
	Render(parentID string, component Component) error

	// Append injects a component AFTER the last child of the parent element.
	// Useful for dynamic lists.
	Append(parentID string, component Component) error

	// OnHashChange registers a listener for changes in the URL hash.
	OnHashChange(handler func(hash string))

	// OnScrollCapture registers a scroll listener in capture phase on the
	// document: fires for any scroller on the page. See the package-level
	// function of the same name.
	OnScrollCapture(handler func(scrollTop float64))

	// OnUserActivity registers document-level presence listeners in the capture
	// phase: the handler is called when the user performs an action anywhere on
	// the page. See the package-level function of the same name.
	OnUserActivity(handler func())

	// GetHash returns the current URL hash (e.g., "#help").
	GetHash() string

	// SetHash updates the URL hash.
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
