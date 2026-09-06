//go:build wasm

package dom

import (
	"syscall/js"

	"webtyp.com/fmt"
)

// domWasm is the WASM implementation of the DOM interface.
type domWasm struct {
	*tinyDOM
	document     js.Value // Cached document object
	localStorage js.Value // Cached localStorage object
	objectCtor   js.Value // Cached global Object constructor, for options objects browser APIs take (scrollIntoView, focus)
	lsUsedBytes  int      // Current localStorage budget usage in bytes (UTF-16)

	elementCache []struct {
		id  string
		val js.Value
	}
	eventFuncs []struct {
		key string
		val js.Value // The element where listener is attached
		fn  js.Func
	}
	componentListeners []struct {
		id   string
		keys []string
	}
	currentComponentID string // Tracks the component being mounted
	pendingEvents      []struct {
		id      string
		ownerID string
		name    string
		handler func(Event)
	}

	// Lifecycle tracking (using slices to avoid map overhead)
	mountedComponents []struct {
		id   string
		comp Component
	}
	childrenMap []struct {
		parentID string
		childIDs []string
	}
	initedIDs []string
	cleanups  []struct {
		id string
		fn func()
	}
	unsubs []struct {
		id    string
		unsub func()
	}
	updating     []string
	rootElements []struct {
		id   string
		root *Element
	}
}

// newDom returns a new instance of the domWasm.
func newDom(td *tinyDOM) DOM {
	ls := js.Global().Get("localStorage")
	used := 0
	if ls.Truthy() {
		// Initial O(n) scan — occurs once at startup to initialize budget counter.
		length := ls.Get("length").Int()
		for i := 0; i < length; i++ {
			key := ls.Call("key", i).String()
			if val := ls.Call("getItem", key); !val.IsNull() && !val.IsUndefined() {
				used += lsEntrySize(key, val.String())
			}
		}
	}
	return &domWasm{
		tinyDOM:      td,
		document:     js.Global().Get("document"),
		localStorage: ls,
		objectCtor:   js.Global().Get("Object"),
		lsUsedBytes:  used,
	}
}

// lsEntrySize estimates the UTF-16 byte size of a localStorage entry.
func lsEntrySize(key, value string) int { return (len(key) + len(value)) * 2 }

// Get retrieves an element by ID from the cache or the DOM.
func (d *domWasm) Get(id string) (Reference, bool) {
	// Linear search in cache
	for i, item := range d.elementCache {
		if item.id == id {
			// Invalidate stale cache if element was removed/replaced
			if !item.val.Get("isConnected").Bool() {
				lastIdx := len(d.elementCache) - 1
				d.elementCache[i] = d.elementCache[lastIdx]
				d.elementCache = d.elementCache[:lastIdx]
				break
			}
			return &elementWasm{
				val: item.val,
				dom: d,
				id:  id,
			}, true
		}
	}

	var val js.Value
	switch id {
	case "body":
		val = d.document.Get("body")
	case "head":
		val = d.document.Get("head")
	default:
		val = d.document.Call("getElementById", id)
	}

	if val.IsNull() || val.IsUndefined() {
		// d.Log("webtyp/dom: element with id", id, "not found")
		return nil, false
	}

	// Append to cache
	d.elementCache = append(d.elementCache, struct {
		id  string
		val js.Value
	}{id, val})

	return &elementWasm{
		val: val,
		dom: d,
		id:  id,
	}, true
}

// getElement resolves a parentID to a js.Value, handling special cases like "body" and "head".
func (d *domWasm) getElement(id string) js.Value {
	switch id {
	case "body":
		return d.document.Get("body")
	case "head":
		return d.document.Get("head")
	default:
		return d.document.Call("getElementById", id)
	}
}

type domCtx struct {
	id string
	d  *domWasm
}

func (c *domCtx) OnCleanup(fn func()) {
	c.d.cleanups = append(c.d.cleanups, struct {
		id string
		fn func()
	}{c.id, fn})
}

// Render injects the component's content into the parent element.
func (d *domWasm) Render(parentID string, component Component) error {
	if d.document.IsNull() || d.document.IsUndefined() {
		return fmt.Errf("document not found")
	}
	// Generate ID if not set
	if component.GetID() == "" {
		component.SetID(generateID())
	}

	d.initComponent(component)

	// Render to HTML and collect child components
	var children []Component
	var html string

	var renderedRoot *Element
	if vr, ok := component.(ViewRenderer); ok {
		root := vr.Render()
		injectComponentID(root, component.GetID())
		html = d.renderToHTML(root, &children, component.GetID())
		renderedRoot = root
	} else if en, ok := component.(elementNode); ok {
		renderedRoot = en.AsElement()
		html = d.renderToHTML(renderedRoot, &children, component.GetID())
	} else if el, ok := component.(*Element); ok {
		renderedRoot = el
		html = d.renderToHTML(renderedRoot, &children, component.GetID())
	} else {
		html = component.String()
	}

	parent := d.getElement(parentID)
	if parent.IsNull() || parent.IsUndefined() {
		return fmt.Errf("parent element not found: %s", parentID)
	}

	// Clean up any existing components in this parent before wiping content.
	d.cleanupChildren(parentID)

	parent.Set("innerHTML", html)

	// Update lifecycle maps
	d.trackComponent(component)
	if renderedRoot != nil {
		d.storeRoot(component.GetID(), renderedRoot)
	}
	d.trackChildren(component.GetID(), children)
	// Register the root component as a direct child of the DOM parent so that
	// a subsequent Render("app", ...) can find and unmount it via cleanupChildren.
	d.trackChildren(parentID, []Component{component})

	// Set current component ID for event wiring
	prevID := d.currentComponentID
	d.currentComponentID = component.GetID()
	d.wirePendingEvents()
	d.wireBindings(component.GetID())
	d.currentComponentID = prevID

	for _, child := range children {
		d.mountRecursive(child)
	}

	if m, ok := component.(mountable); ok {
		m.Mounted()
	}

	return nil
}

func (d *domWasm) initComponent(c Component) {
	if c == nil {
		return
	}
	id := c.GetID()
	for _, initedID := range d.initedIDs {
		if initedID == id {
			return
		}
	}

	if initable, ok := c.(initable); ok {
		initable.Init(&domCtx{id: id, d: d})
	}
	d.initedIDs = append(d.initedIDs, id)
}

// update re-renders the component and replaces it in the DOM.
func (d *domWasm) update(id string) {
	if d.document.IsNull() || d.document.IsUndefined() {
		d.Log("webtyp/dom: document not found in update")
		return
	}

	for _, uid := range d.updating {
		if uid == id {
			if d.devMode {
				d.Log("webtyp/dom: re-entrant update on", id, "ignored")
			}
			return
		}
	}
	d.updating = append(d.updating, id)

	var component Component
	// Resolve the full outer component from tracked references.
	for _, item := range d.mountedComponents {
		if item.id == id {
			component = item.comp
			break
		}
	}

	if component == nil {
		// Remove from updating before returning
		for i, uid := range d.updating {
			if uid == id {
				d.updating = append(d.updating[:i], d.updating[i+1:]...)
				break
			}
		}
		return
	}

	// Clean up old children listeners/lifecycle
	d.cleanupChildren(id)
	d.cleanupListeners(id)

	var children []Component
	var html string

	if vr, ok := component.(ViewRenderer); ok {
		root := vr.Render()
		injectComponentID(root, id)
		html = d.renderToHTML(root, &children, id)
	} else if en, ok := component.(elementNode); ok {
		html = d.renderToHTML(en.AsElement(), &children, id)
	} else if el, ok := component.(*Element); ok {
		html = d.renderToHTML(el, &children, id)
	} else {
		html = component.String()
	}

	// Replace the element in the DOM
	elRaw := d.document.Call("getElementById", id)
	if elRaw.IsNull() || elRaw.IsUndefined() {
		if d.devMode {
			d.Log("webtyp/dom: component element not found during update:", id, "(this usually means the component root element has no ID)")
		}
		// Remove from updating before returning
		for i, uid := range d.updating {
			if uid == id {
				d.updating = append(d.updating[:i], d.updating[i+1:]...)
				break
			}
		}
		return
	}

	// Snapshot active element and cursor before outerHTML destroys them.
	activeEl := d.document.Get("activeElement")
	activeID := ""
	cursorStart, cursorEnd := 0, 0
	if !activeEl.IsNull() && !activeEl.IsUndefined() {
		activeID = activeEl.Get("id").String()
		cs := activeEl.Get("selectionStart")
		ce := activeEl.Get("selectionEnd")
		if !cs.IsNull() && !cs.IsUndefined() {
			cursorStart = cs.Int()
		}
		if !ce.IsNull() && !ce.IsUndefined() {
			cursorEnd = ce.Int()
		}
	}

	elRaw.Set("outerHTML", html)

	// Clear element from cache as it was replaced
	d.removeFromElementCache(id)

	// Update lifecycle maps
	d.trackChildren(id, children)

	// Set current component ID for event wiring
	prevID := d.currentComponentID
	d.currentComponentID = id
	d.wirePendingEvents()
	d.wireBindings(id)
	d.currentComponentID = prevID

	// Mount new children
	for _, child := range children {
		d.mountRecursive(child)
	}

	if m, ok := component.(mountable); ok {
		m.Mounted()
	}

	// Restore focus and cursor to the element that was active before outerHTML replacement.
	if activeID != "" {
		restored := d.document.Call("getElementById", activeID)
		if !restored.IsNull() && !restored.IsUndefined() {
			currentActive := d.document.Get("activeElement")
			alreadyActive := !currentActive.IsNull() && !currentActive.IsUndefined() &&
				currentActive.Get("id").String() == activeID
			if !alreadyActive {
				restored.Call("focus")
			}
			cs := restored.Get("selectionStart")
			if !cs.IsNull() && !cs.IsUndefined() {
				restored.Call("setSelectionRange", cursorStart, cursorEnd)
			}
		}
	}

	for i, uid := range d.updating {
		if uid == id {
			d.updating = append(d.updating[:i], d.updating[i+1:]...)
			break
		}
	}
}

// Update re-renders the component (NOT public anymore, but needed for internal reasons? No, PLAN says unexport)
// ActuallyPLAN says: "Update re-renders a component." -> was dom.Update(comp).
// PLAN Change 4 says: "Unexport Update -> update (an internal primitive used by Show/BindChildren...)"

// Append injects the component's content after the last child of the parent element.
func (d *domWasm) Append(parentID string, component Component) error {
	if component.GetID() == "" {
		component.SetID(generateID())
	}

	d.initComponent(component)

	var children []Component
	var html string
	var appendRoot *Element
	if vr, ok := component.(ViewRenderer); ok {
		root := vr.Render()
		injectComponentID(root, component.GetID())
		html = d.renderToHTML(root, &children, component.GetID())
		appendRoot = root
	} else if en, ok := component.(elementNode); ok {
		appendRoot = en.AsElement()
		html = d.renderToHTML(appendRoot, &children, component.GetID())
	} else if el, ok := component.(*Element); ok {
		appendRoot = el
		html = d.renderToHTML(appendRoot, &children, component.GetID())
	} else {
		html = component.String()
	}

	parent := d.getElement(parentID)
	if parent.IsNull() || parent.IsUndefined() {
		return fmt.Errf("parent element not found: %s", parentID)
	}
	parent.Call("insertAdjacentHTML", "beforeend", html)

	d.trackComponent(component)
	if appendRoot != nil {
		d.storeRoot(component.GetID(), appendRoot)
	}
	d.trackChildren(component.GetID(), children)

	prevID := d.currentComponentID
	d.currentComponentID = component.GetID()
	d.wirePendingEvents()
	d.wireBindings(component.GetID())
	d.currentComponentID = prevID

	for _, child := range children {
		d.mountRecursive(child)
	}

	if m, ok := component.(mountable); ok {
		m.Mounted()
	}

	return nil
}

// unmount removes a component from the DOM and recursively cleans up children.
func (d *domWasm) unmount(component Component) {
	d.unmountRecursive(component)

	// Remove the element from the DOM
	id := component.GetID()
	el := d.document.Call("getElementById", id)
	if !el.IsNull() && !el.IsUndefined() {
		el.Call("remove")
	}

	d.removeFromElementCache(id)
	d.untrackComponent(id)
}

func (d *domWasm) renderToHTML(el *Element, comps *[]Component, ownerID string) string {
	if el == nil {
		if d.devMode {
			d.Log("webtyp/dom: nil Element encountered during renderToHTML (pointer-embedded Element mistake?)")
		}
		return ""
	}

	renderChild := func(v Component) string {
		if v == nil {
			if d.devMode {
				d.Log("webtyp/dom: nil Component encountered (pointer-embedded Element mistake?)")
			}
			return ""
		}
		*comps = append(*comps, v)
		if v.GetID() == "" {
			v.SetID(generateID())
		}
		childID := v.GetID()
		d.initComponent(v)

		if vr, ok := v.(ViewRenderer); ok {
			root := vr.Render()
			injectComponentID(root, childID)
			s := d.renderToHTML(root, comps, childID)
			d.storeRoot(childID, root)
			return s
		} else if en, ok := v.(elementNode); ok {
			root := en.AsElement()
			injectComponentID(root, childID)
			s := d.renderToHTML(root, comps, childID)
			d.storeRoot(childID, root)
			return s
		} else if el, ok := v.(*Element); ok {
			injectComponentID(el, childID)
			s := d.renderToHTML(el, comps, childID)
			d.storeRoot(childID, el)
			return s
		} else {
			return v.String()
		}
	}

	observer := func(elem *Element) {
		for _, ev := range elem.events {
			d.pendingEvents = append(d.pendingEvents, struct {
				id      string
				ownerID string
				name    string
				handler func(Event)
			}{elem.id, ownerID, ev.Name, ev.Handler})
		}
	}

	return serializeElement(el, renderChild, observer)
}

func (d *domWasm) mountRecursive(c Component) {
	if c == nil {
		return
	}
	prevID := d.currentComponentID
	d.currentComponentID = c.GetID()
	defer func() { d.currentComponentID = prevID }()

	// Wire bindings for this child component using the stored root from renderToHTML.
	d.wireBindings(c.GetID())

	for _, child := range c.Children() {
		if child != nil {
			d.mountRecursive(child)
		}
	}

	if m, ok := c.(mountable); ok {
		m.Mounted()
	}
}

func (d *domWasm) unmountRecursive(c Component) {
	if c == nil {
		return
	}
	// Cleanup children first
	// 1. From Children() interface
	for _, child := range c.Children() {
		if child != nil {
			d.unmountRecursive(child)
		}
	}

	// 2. From tracked children map
	id := c.GetID()
	var childIDs []string
	for i, item := range d.childrenMap {
		if item.parentID == id {
			childIDs = item.childIDs
			// Remove from map
			lastIdx := len(d.childrenMap) - 1
			d.childrenMap[i] = d.childrenMap[lastIdx]
			d.childrenMap = d.childrenMap[:lastIdx]
			break
		}
	}

	for _, childID := range childIDs {
		// Find component instance
		var childComp Component
		for i, item := range d.mountedComponents {
			if item.id == childID {
				childComp = item.comp
				// Remove from mounted
				lastIdx := len(d.mountedComponents) - 1
				d.mountedComponents[i] = d.mountedComponents[lastIdx]
				d.mountedComponents = d.mountedComponents[:lastIdx]
				break
			}
		}
		if childComp != nil {
			d.unmountRecursive(childComp)
		} else {
			// If instance not found, just cleanup listeners
			d.cleanupListeners(childID)
			d.cleanupSignalSubscriptions(childID)
			d.runCleanups(childID)
		}
	}

	d.cleanupListeners(c.GetID())
	d.cleanupSignalSubscriptions(c.GetID())
	d.runCleanups(c.GetID())
	d.untrackComponent(c.GetID())
	d.clearRoot(c.GetID())

	// Remove from initedIDs so it can be re-inited if re-mounted
	for i, initedID := range d.initedIDs {
		if initedID == c.GetID() {
			d.initedIDs = append(d.initedIDs[:i], d.initedIDs[i+1:]...)
			break
		}
	}
}

func (d *domWasm) cleanupChildren(parentID string) {
	// Similar to unmountRecursive but only for tracked children
	var childIDs []string
	for i, item := range d.childrenMap {
		if item.parentID == parentID {
			childIDs = item.childIDs
			lastIdx := len(d.childrenMap) - 1
			d.childrenMap[i] = d.childrenMap[lastIdx]
			d.childrenMap = d.childrenMap[:lastIdx]
			break
		}
	}

	for _, childID := range childIDs {
		var childComp Component
		for i, item := range d.mountedComponents {
			if item.id == childID {
				childComp = item.comp
				lastIdx := len(d.mountedComponents) - 1
				d.mountedComponents[i] = d.mountedComponents[lastIdx]
				d.mountedComponents = d.mountedComponents[:lastIdx]
				break
			}
		}
		if childComp != nil {
			d.unmountRecursive(childComp)
		} else {
			d.cleanupListeners(childID)
		}
	}
}

// Helpers for state management

func (d *domWasm) trackComponent(c Component) {
	id := c.GetID()
	for _, item := range d.mountedComponents {
		if item.id == id {
			return // Already tracked
		}
	}
	d.mountedComponents = append(d.mountedComponents, struct {
		id   string
		comp Component
	}{id, c})
}

func (d *domWasm) storeRoot(id string, root *Element) {
	for i, item := range d.rootElements {
		if item.id == id {
			d.rootElements[i].root = root
			return
		}
	}
	d.rootElements = append(d.rootElements, struct {
		id   string
		root *Element
	}{id, root})
}

func (d *domWasm) loadRoot(id string) (*Element, bool) {
	for _, item := range d.rootElements {
		if item.id == id {
			return item.root, true
		}
	}
	return nil, false
}

func (d *domWasm) clearRoot(id string) {
	for i, item := range d.rootElements {
		if item.id == id {
			last := len(d.rootElements) - 1
			d.rootElements[i] = d.rootElements[last]
			d.rootElements = d.rootElements[:last]
			return
		}
	}
}

func (d *domWasm) untrackComponent(id string) {
	for i, item := range d.mountedComponents {
		if item.id == id {
			lastIdx := len(d.mountedComponents) - 1
			d.mountedComponents[i] = d.mountedComponents[lastIdx]
			d.mountedComponents = d.mountedComponents[:lastIdx]
			break
		}
	}
}

func (d *domWasm) trackChildren(parentID string, children []Component) {
	var childIDs []string
	for _, c := range children {
		if c == nil {
			continue
		}
		childIDs = append(childIDs, c.GetID())
		d.trackComponent(c)
	}

	// Check if entry exists
	found := false
	for i, item := range d.childrenMap {
		if item.parentID == parentID {
			d.childrenMap[i].childIDs = childIDs
			found = true
			break
		}
	}
	if !found {
		d.childrenMap = append(d.childrenMap, struct {
			parentID string
			childIDs []string
		}{parentID, childIDs})
	}
}

func (d *domWasm) removeFromElementCache(id string) {
	for i, item := range d.elementCache {
		if item.id == id {
			lastIdx := len(d.elementCache) - 1
			d.elementCache[i] = d.elementCache[lastIdx]
			d.elementCache = d.elementCache[:lastIdx]
			break
		}
	}
}

func (d *domWasm) wirePendingEvents() {
	for _, pe := range d.pendingEvents {
		if el, ok := d.Get(pe.id); ok {
			// Track listener for the component that owns the element
			prev := d.currentComponentID
			d.currentComponentID = pe.ownerID
			el.On(pe.name, pe.handler)
			d.currentComponentID = prev
		}
	}
	d.pendingEvents = nil
}

func (d *domWasm) cleanupListeners(id string) {
	var keysToRemove []string
	compIndex := -1
	for i, item := range d.componentListeners {
		if item.id == id {
			keysToRemove = item.keys
			compIndex = i
			break
		}
	}

	if compIndex != -1 {
		for _, key := range keysToRemove {
			// Find and remove from eventFuncs
			for i := 0; i < len(d.eventFuncs); i++ {
				ef := d.eventFuncs[i]
				if ef.key == key {
					// Split key into id::type
					parts := d.splitEventKey(key)
					if len(parts) == 2 && !ef.val.IsNull() && !ef.val.IsUndefined() {
						eventType := parts[1]
						ef.val.Call("removeEventListener", eventType, ef.fn)
					}
					ef.fn.Release()

					// Remove from slice
					lastIdx := len(d.eventFuncs) - 1
					d.eventFuncs[i] = d.eventFuncs[lastIdx]
					d.eventFuncs = d.eventFuncs[:lastIdx]
					i--
				}
			}
		}
		// Remove from componentListeners
		lastIdx := len(d.componentListeners) - 1
		d.componentListeners[compIndex] = d.componentListeners[lastIdx]
		d.componentListeners = d.componentListeners[:lastIdx]
	}
}

func (d *domWasm) wireBindings(id string) {
	// Use the root stored during the first Render() call so IDs match the DOM.
	// Calling Render() again would generate new auto-IDs that don't exist in the DOM,
	// making all BindText/BindAttr/BindChildren subscriptions target phantom elements.
	if root, ok := d.loadRoot(id); ok {
		d.wireElementBindings(root, id)
		return
	}

	// Fallback for elementNode / *Element components (no ViewRenderer).
	var component Component
	for _, item := range d.mountedComponents {
		if item.id == id {
			component = item.comp
			break
		}
	}
	if component == nil {
		return
	}
	if en, ok := component.(elementNode); ok {
		d.wireElementBindings(en.AsElement(), id)
	} else if el, ok := component.(*Element); ok {
		d.wireElementBindings(el, id)
	}
}

// boolPropName maps a boolean content attribute to its live IDL property name.
// These properties drift from the content attribute after user interaction, so a
// signal-driven attrbool binding must set the property, not just the attribute.
func boolPropName(attr string) (string, bool) {
	switch attr {
	case "checked":
		return "checked", true
	case "disabled":
		return "disabled", true
	case "selected":
		return "selected", true
	case "multiple":
		return "multiple", true
	case "open":
		return "open", true
	case "hidden":
		return "hidden", true
	case "readonly":
		return "readOnly", true
	default:
		return "", false
	}
}

func (d *domWasm) wireElementBindings(el *Element, ownerID string) {
	if el == nil {
		return
	}
	if el.autofocus {
		if ref, ok := d.Get(el.id); ok {
			// Focus iff nothing else is focused
			activeEl := d.document.Get("activeElement")
			if activeEl.IsNull() || activeEl.IsUndefined() || activeEl.Get("tagName").String() == "BODY" {
				ref.Focus()
			}
		}
	}

	for _, b := range el.bindings {
		b := b
		ref, ok := d.Get(el.id)
		if !ok {
			continue
		}

		var updater func()
		switch b.kind {
		case "text":
			updater = func() {
				val := ""
				if b.signal != nil {
					if sig, ok := b.signal.(*SignalString); ok {
						val = sig.Get()
					}
				} else if b.fnString != nil {
					val = b.fnString()
				}
				ref.SetText(val)
				if d.devMode {
					d.Log("[dom] patch #"+el.id+" textContent:", val)
				}
			}
		case "attr":
			updater = func() {
				val := ""
				if b.signal != nil {
					if sig, ok := b.signal.(*SignalString); ok {
						val = sig.Get()
					}
				} else if b.fnString != nil {
					val = b.fnString()
				}
				ref.SetAttr(b.name, val)
				if d.devMode {
					d.Log("[dom] patch #"+el.id+" attr "+b.name+":", val)
				}
			}
		case "class":
			updater = func() {
				on := false
				if b.signal != nil {
					if sig, ok := b.signal.(*SignalBool); ok {
						on = sig.Get()
					}
				} else if b.fnBool != nil {
					on = b.fnBool()
				}
				if on {
					ref.(*elementWasm).val.Get("classList").Call("add", b.name)
				} else {
					ref.(*elementWasm).val.Get("classList").Call("remove", b.name)
				}
				if d.devMode {
					d.Log("[dom] patch #"+el.id+" class "+b.name+":", on)
				}
			}
		case "attrbool":
			updater = func() {
				on := false
				if b.signal != nil {
					if sig, ok := b.signal.(*SignalBool); ok {
						on = sig.Get()
					}
				} else if b.fnBool != nil {
					on = b.fnBool()
				}
				if on {
					ref.SetAttr(b.name, "")
				} else {
					ref.RemoveAttr(b.name)
				}
				// Boolean form attributes (checked, disabled, selected, ...) have a live
				// IDL property that does NOT follow the content attribute once the user has
				// interacted with the control. Setting only the attribute leaves the property
				// stale: e.g. a checkbox bound to a signal stays .checked==true after the
				// attribute is removed, so the next user click toggles property→false and the
				// "change" event reports the wrong state (requiring a second click). Keep the
				// property in sync explicitly.
				if prop, ok := boolPropName(b.name); ok {
					ref.(*elementWasm).val.Set(prop, on)
				}
				if d.devMode {
					d.Log("[dom] patch #"+el.id+" attrbool "+b.name+":", on)
				}
			}
		case "state":
			updater = func() {
				on := false
				if b.signal != nil {
					if sig, ok := b.signal.(*SignalBool); ok {
						on = sig.Get()
					}
				} else if b.fnBool != nil {
					on = b.fnBool()
				}
				if on {
					ref.SetAttr(b.state.Key(), b.state.Value())
				} else {
					ref.RemoveAttr(b.state.Key())
				}
				if d.devMode {
					d.Log("[dom] patch #"+el.id+" state "+b.state.Key()+":", on)
				}
			}
		case "value":
			// Two-way binding
			if b.signal == nil {
				continue
			}
			sig, ok := b.signal.(*SignalString)
			if !ok {
				continue
			}

			// Check if element is input/textarea
			tagName := ref.(*elementWasm).val.Get("tagName").String()
			if tagName != "INPUT" && tagName != "TEXTAREA" {
				if d.devMode {
					d.Log("webtyp/dom: Bind used on non-input element:", tagName)
				}
			}

			updater = func() {
				val := sig.Get()
				// Skip if activeElement to avoid cursor jumps
				activeEl := d.document.Get("activeElement")
				if !activeEl.IsNull() && !activeEl.IsUndefined() && activeEl.Get("id").String() == el.id {
					return
				}
				if ref.Value() != val {
					ref.SetValue(val)
				}
			}

			// Listen for input changes
			ref.On("input", func(e Event) {
				sig.Set(ref.Value())
			})
		case "children":
			sig, ok := b.signal.(*SignalNodes)
			if !ok {
				continue
			}
			// Rows already present in the signal at first render are serialized
			// straight into the parent's HTML (see the "children" case in
			// renderToHTML) and never pass through reconcileChildren, so their own
			// nested bindings (BindClass/BindText/…) would be left unwired — the
			// row appears but never reacts. Wire them now, using el.id as the owner
			// exactly like reconcileChildren does for rows it inserts later.
			for _, n := range sig.Get() {
				d.wireElementBindings(n, el.id)
			}
			updater = func() {
				d.reconcileChildren(el.id, sig.Get())
			}
		}

		if updater != nil {
			if b.signal != nil {
				unsub := b.signal.subscribe(updater)
				d.unsubs = append(d.unsubs, struct {
					id    string
					unsub func()
				}{ownerID, unsub})
			} else if b.fnString != nil || b.fnBool != nil {
				// Use tracker for computed bindings
				t := &tracker{}
				prev := currentTracker
				currentTracker = t
				updater() // Initial run to track
				currentTracker = prev

				for _, s := range t.signals {
					unsub := s.subscribe(updater)
					d.unsubs = append(d.unsubs, struct {
						id    string
						unsub func()
					}{ownerID, unsub})
				}
			}
		}
	}

	for _, child := range el.children {
		if childEl, ok := child.(*Element); ok {
			d.wireElementBindings(childEl, ownerID)
		}
	}
}

func (d *domWasm) cleanupSignalSubscriptions(id string) {
	for i := 0; i < len(d.unsubs); i++ {
		if d.unsubs[i].id == id {
			d.unsubs[i].unsub()
			d.unsubs = append(d.unsubs[:i], d.unsubs[i+1:]...)
			i--
		}
	}
}

func (d *domWasm) runCleanups(id string) {
	for i := 0; i < len(d.cleanups); i++ {
		if d.cleanups[i].id == id {
			d.cleanups[i].fn()
			d.cleanups = append(d.cleanups[:i], d.cleanups[i+1:]...)
			i--
		}
	}
}

func (d *domWasm) reconcileChildren(parentID string, newNodes []*Element) {
	parent, ok := d.Get(parentID)
	if !ok {
		return
	}
	parentVal := parent.(*elementWasm).val

	// Keyed reconcile
	existingNodes := parentVal.Get("children")
	existingLen := existingNodes.Get("length").Int()

	// Build map of current children by key
	currentKeys := make([]struct {
		key string
		val js.Value
	}, existingLen)
	for i := 0; i < existingLen; i++ {
		node := existingNodes.Call("item", i)
		currentKeys[i] = struct {
			key string
			val js.Value
		}{node.Get("id").String(), node}
	}

	// Dev mode key validation
	if d.devMode {
		keys := make([]string, 0, len(newNodes))
		for _, n := range newNodes {
			if n.key == "" && n.id == "" {
				d.Log("webtyp/dom: row in BindChildren has no key/id (volatile identity)")
			}
			key := n.key
			if key == "" {
				key = n.id
			}
			for _, existingKey := range keys {
				if key != "" && existingKey == key {
					d.Log("webtyp/dom: duplicate key in BindChildren:", key)
				}
			}
			keys = append(keys, key)
		}
	}

	// Simplistic reconciliation: for each new node, if it exists, move it; otherwise insert it.
	// Then remove any old nodes that aren't in the new set.
	var comps []Component
	for i, n := range newNodes {
		key := n.key
		if key == "" {
			key = n.id
		}
		if key == "" {
			key = generateID()
			n.id = key
		}

		var found js.Value
		for _, item := range currentKeys {
			if item.key == key {
				found = item.val
				break
			}
		}

		if !found.IsUndefined() && !found.IsNull() {
			// Move to correct position if needed
			if i < existingLen && !existingNodes.Call("item", i).Equal(found) {
				parentVal.Call("insertBefore", found, existingNodes.Call("item", i))
			}
		} else {
			// Create and insert
			html := d.renderToHTML(n, &comps, parentID)
			tempDiv := d.document.Call("createElement", "div")
			tempDiv.Set("innerHTML", html)
			newNode := tempDiv.Get("firstElementChild")
			if i < existingLen {
				parentVal.Call("insertBefore", newNode, existingNodes.Call("item", i))
			} else {
				parentVal.Call("appendChild", newNode)
			}
			// Wire bindings and events for the new node only
			d.wireElementBindings(n, parentID)
		}
	}

	// Remove extra nodes. Uses the live child count, not the frozen snapshot
	// taken before insertions: when an existing node's key does not match any
	// new node's key, insertBefore creates a sibling without removing the old
	// one, so the live collection is longer than both existingLen and newNodes.
	for parentVal.Get("children").Get("length").Int() > len(newNodes) {
		last := parentVal.Get("lastElementChild")
		lastID := last.Get("id").String()
		last.Call("remove")
		d.cleanupListeners(lastID)
		d.cleanupSignalSubscriptions(lastID)
		d.runCleanups(lastID)
	}

	d.wirePendingEvents()
	for _, c := range comps {
		d.mountRecursive(c)
	}
}

// Show keeps content mounted and toggles its visibility with cond.
// The subtree is built and attached ONCE — a builder re-run that re-attaches
// captured elements (the v0.12 panic) is unrepresentable: there is no builder.
// Hidden means inline display:none on the container, so node identity,
// listeners and signal bindings survive every toggle, and bindings keep
// patching while hidden — the subtree is current the moment it reappears.
func Show(cond *SignalBool, content Component) *Element {
	containerID := generateID()
	container := NewElement("div").ID(containerID)
	if !cond.Get() {
		container.Attr("style", "display:none")
	}
	container.Child(content)

	updater := func() {
		if ref, ok := instance.Get(containerID); ok {
			display := ""
			if !cond.Get() {
				display = "none"
			}
			ref.(*elementWasm).val.Get("style").Set("display", display)
		}
	}
	unsub := cond.subscribe(updater)

	// Register unsub to be called when container is unmounted
	instance.(*domWasm).unsubs = append(instance.(*domWasm).unsubs, struct {
		id    string
		unsub func()
	}{containerID, unsub})

	return container
}

func (d *domWasm) splitEventKey(key string) []string {
	// Simple manual split to avoid importing strings just for this
	for i := 0; i < len(key)-1; i++ {
		if key[i] == ':' && key[i+1] == ':' {
			return []string{key[:i], key[i+2:]}
		}
	}
	return nil
}

// OnHashChange registers a listener for window.hashchange.
func (d *domWasm) OnHashChange(handler func(hash string)) {
	fn := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		handler(d.GetHash())
		return nil
	})
	js.Global().Get("window").Call("addEventListener", "hashchange", fn)
}

// OnScrollCapture registra el listener en el documento con capture=true, que es lo
// que permite ver el scroll de descendientes: el evento scroll no burbujea, pero
// sí baja por la fase de captura.
func (d *domWasm) OnScrollCapture(handler func(scrollTop float64)) {
	fn := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		target := args[0].Get("target")
		// document.scrollingElement cuando el que se desplaza es el documento
		// mismo: ahí el target es el Document, que no tiene scrollTop.
		top := target.Get("scrollTop")
		if top.IsUndefined() || top.IsNull() {
			top = js.Global().Get("document").Get("scrollingElement").Get("scrollTop")
		}
		handler(top.Float())
		return nil
	})
	js.Global().Get("document").Call("addEventListener", "scroll", fn, true)
}

// GetHash returns current window.location.hash.
func (d *domWasm) GetHash() string {
	return js.Global().Get("location").Get("hash").String()
}

// SetHash updates window.location.hash.
func (d *domWasm) SetHash(hash string) {
	js.Global().Get("location").Set("hash", hash)
}
