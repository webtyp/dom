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

// OnHashChange registers a hash change listener.
func OnHashChange(handler func(hash string)) {
	instance.OnHashChange(handler)
}

// OnScrollCapture registers a scroll listener in CAPTURE PHASE on the
// document, so that it fires for ANY scroller on the page, not just
// for the window.
//
// It exists because the scroll event does not bubble: it fires only on the element
// that scrolled. A shell that wants to react to the scroll of its content cannot
// know which descendant of which component actually overflows, and registering
// the listener element by element would force it to know the internal implementation
// of other packages.
//
// scrollTop is the vertical position of the element that fired the event. With multiple
// scrollers on screen, values interleave: callers comparing positions must tolerate
// this with a threshold, rather than assuming a continuous series.
//
// There is no way to unregister it: it is a document listener that lives for the lifetime
// of the page.
func OnScrollCapture(handler func(scrollTop float64)) {
	instance.OnScrollCapture(handler)
}

// OnUserActivity registers document-level capture-phase listeners that represent
// "a human is present": pointer movement and press (mouse, finger, or pen), keypress,
// wheel, and scroll. The handler is called when any of these occur anywhere on the page.
//
// It exists for the same reason as OnScrollCapture: presence is a DOCUMENT fact,
// not an element fact. Attaching listeners to the root element of a component fails in
// three ways — mouseenter fires ONCE upon crossing the boundary and does not repeat with
// movement, keydown only arrives if focus is inside that subtree, and anything mounted
// outside the subtree counts for nothing. All three failure modes disappear in capture phase
// on the document.
//
// The handler receives no parameters: the only datum is that activity occurred. It is called
// at most once per second — pointermove fires at refresh rate and this is a presence signal,
// not an event stream. Anything measuring inactivity operates in seconds, so the 1-second
// window is unnoticeable.
//
// dom has no concept of inactivity: there is no timer, no threshold, and no concept of "idle"
// here. How much time counts as idle, and what happens then, belongs to the consumer.
//
// Register ONCE: there is no way to unregister it, these are document listeners that live for
// the lifetime of the page. Two calls register two sets of listeners, each with its own
// 1-second throttling window.
func OnUserActivity(handler func()) {
	instance.OnUserActivity(handler)
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
