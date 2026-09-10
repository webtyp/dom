package dom

// Key is the value of a keyboard event's key: the DOM's own
// KeyboardEvent.key, verbatim ("ArrowLeft", " ", "Enter"). A string-backed
// named type, so a typo'd literal does not compile where a Key is required
// and the zero value is meaningless rather than a valid key.
//
// Exactly the twelve keys the ecosystem's widgets navigate with. A key not
// in the set stays reachable through the raw value a KeyEvent exposes;
// adding a constant is a one-line change here, in the package that owns DOM
// events — never a string literal at a call site.
type Key string

const (
	KeyArrowLeft  Key = "ArrowLeft"
	KeyArrowRight Key = "ArrowRight"
	KeyArrowUp    Key = "ArrowUp"
	KeyArrowDown  Key = "ArrowDown"
	KeyHome       Key = "Home"
	KeyEnd        Key = "End"
	KeyPageUp     Key = "PageUp"
	KeyPageDown   Key = "PageDown"
	KeyEnter      Key = "Enter"
	KeySpace      Key = " "
	KeyEscape     Key = "Escape"
	KeyTab        Key = "Tab"
)
