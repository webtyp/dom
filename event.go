package dom

// Event represents a DOM event.
type Event interface {
	// PreventDefault prevents the default action of the event.
	PreventDefault()
	// StopPropagation stops the event from bubbling up the DOM tree.
	StopPropagation()
	// TargetValue returns the value of the event's target element.
	// Useful for input, textarea, and select elements.
	TargetValue() string
	// TargetID returns the ID of the event's target element.
	TargetID() string
	// TargetChecked returns the checked status of the event's target element.
	// Useful for checkbox and radio input elements.
	TargetChecked() bool
}

// KeyEvent is what a keyboard handler receives: an Event plus the key.
//
// Key() is NOT on Event. A click carrying Key() == "" would be an illegal
// state made representable; the handler's parameter is narrowed by the event
// instead — OnKeyDown takes func(KeyEvent), every other On* takes
// func(Event).
type KeyEvent interface {
	Event
	// Key returns the pressed key: the DOM's KeyboardEvent.key verbatim.
	Key() Key
}
