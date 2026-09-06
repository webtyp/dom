# FEAT_003: Debounce Config

> **Status:** Planned  
> **Priority:** P1

## Overview

Global debounce configuration for form validation and other timed operations.

## Configuration

```go
// config.go

// Config holds global webtyp/dom configuration
type Config struct {
    DebounceMs int // Default: 200ms
}

var globalConfig = Config{
    DebounceMs: 200,
}

// SetConfig sets global configuration
func SetConfig(cfg Config) {
    if cfg.DebounceMs > 0 {
        globalConfig.DebounceMs = cfg.DebounceMs
    }
}

// GetDebounceMs returns current debounce setting
func GetDebounceMs() int {
    return globalConfig.DebounceMs
}
```

## Debounce Utility

```go
// debounce.go
//go:build wasm

import "syscall/js"

// Debounce creates a debounced function
// Returns a function that delays invoking fn until after ms milliseconds
// have elapsed since the last time it was invoked
func Debounce(ms int, fn func()) func() {
    var timeoutID js.Value
    
    return func() {
        // Clear existing timeout
        if !timeoutID.IsUndefined() && !timeoutID.IsNull() {
            js.Global().Call("clearTimeout", timeoutID)
        }
        
        // Set new timeout
        callback := js.FuncOf(func(this js.Value, args []js.Value) any {
            fn()
            return nil
        })
        
        timeoutID = js.Global().Call("setTimeout", callback, ms)
    }
}

// DebounceDefault uses global config debounce time
func DebounceDefault(fn func()) func() {
    return Debounce(globalConfig.DebounceMs, fn)
}
```

## Backend Stub

```go
// debounce_backend.go
//go:build !wasm

// Debounce is a no-op in backend (SSR)
func Debounce(ms int, fn func()) func() {
    return fn // Just return the function as-is
}

func DebounceDefault(fn func()) func() {
    return fn
}
```

## Usage

```go
// In form validation (internal)
debouncedValidate := webtyp/dom.Debounce(config.DebounceMs, func() {
    err := validator.ValidateField(fieldName, input.Value())
    // Update UI...
})

input.OnInput(func(e Event) {
    debouncedValidate() // Calls validate after debounce
})
```

```go
// User can also use directly
func (h *Handler) OnMount(dom webtyp/dom.DOM) {
    input, _ := dom.Get("search")
    
    search := webtyp/dom.DebounceDefault(func() {
        query := input.Value()
        dom.Send(webtyp/dom.GET, h, Query{Text: query}, h.onSearchResults)
    })
    
    input.OnInput(func(e webtyp/dom.Event) {
        search()
    })
}
```

## Files to Create

- `config.go`: Global config
- `debounce.go`: WASM implementation
- `debounce_backend.go`: Backend stub

## Notes

- Uses JS `setTimeout`/`clearTimeout` for WASM
- No goroutines needed
- Backend returns function as-is (no debounce in SSR)
