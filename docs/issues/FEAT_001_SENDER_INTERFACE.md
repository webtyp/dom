# FEAT_001: Sender Interface

> **Status:** Planned  
> **Priority:** P0

## Overview

Add a `Sender` interface to DOM for making network requests. The implementation is injected at initialization, allowing CRUDP (or any other transport) to handle the actual communication.

## Interface Definition

```go
// interfaces.go

// NamedHandler identifies a handler by name
type NamedHandler interface {
    HandlerName() string
}

// Sender sends requests to the server
// Implementation is injected (e.g., CRUDP broker)
type Sender interface {
    Send(method string, handler NamedHandler, data any, callback func([]byte, error))
}
```

## HTTP Method Constants

```go
// methods.go
const (
    POST   = "POST"   // Create
    GET    = "GET"    // Read
    PUT    = "PUT"    // Update
    DELETE = "DELETE" // Delete
)
```

## DOM Interface Update

```go
// dom.go
type DOM interface {
    // Existing methods
    Get(id string) (Element, bool)
    Render(parentID string, component Component) error
    Unmount(component Component)
    Log(v ...any)
    
    // New: Send request via injected Sender
    Send(method string, handler NamedHandler, data any, callback func([]byte, error))
}
```

## Implementation

```go
// dom_frontend.go
//go:build wasm

type domImpl struct {
    log    func(...any)
    sender Sender
    // ... existing fields
}

func (d *domImpl) Send(method string, handler NamedHandler, data any, callback func([]byte, error)) {
    if d.sender == nil {
        if callback != nil {
            callback(nil, tinystring.Err("sender not configured"))
        }
        return
    }
    d.sender.Send(method, handler, data, callback)
}
```

## New() Update

```go
// dom_frontend.go

// New creates a DOM instance
// logger: optional logging function
// sender: optional Sender implementation (e.g., CRUDP)
func New(logger func(...any), sender Sender) DOM {
    return &domImpl{
        log:    logger,
        sender: sender,
    }
}
```

## Usage Example

```go
// main.go (wasm)
func main() {
    // CRUDP provides Sender implementation
    cp := crudp.New(cfg)
    cp.RegisterHandler(&user.Handler{})
    
    // Pass CRUDP as Sender to webtyp/dom
    dom := webtyp/dom.New(log.Println, cp)
    
    // Render components
    dom.Render("app", myComponent)
}
```

```go
// user_front.go (handler)
func (h *Handler) OnMount(dom webtyp/dom.DOM) {
    btn, _ := dom.Get("save-btn")
    btn.Click(func(e webtyp/dom.Event) {
        user := User{Name: "John"}
        dom.Send(webtyp/dom.POST, h, user, func(resp []byte, err error) {
            if err != nil {
                dom.Log("Error:", err)
                return
            }
            // Response handled by broker -> handler.Create()
        })
    })
}
```

## Files to Modify

- `interfaces.go`: Add NamedHandler, Sender interfaces
- `methods.go`: Create with HTTP constants
- `dom.go`: Add Send to DOM interface
- `dom_frontend.go`: Implement Send, update New()
- `dom_backend.go`: Add no-op Send for SSR

## Backend (SSR) Implementation

```go
// dom_backend.go
//go:build !wasm

func (d *domImpl) Send(method string, handler NamedHandler, data any, callback func([]byte, error)) {
    // No-op in SSR, or panic if called
    if callback != nil {
        callback(nil, tinystring.Err("Send not available in SSR"))
    }
}
```

## Tests

```go
// sender_test.go
type mockSender struct {
    lastMethod  string
    lastHandler string
    lastData    any
}

func (m *mockSender) Send(method string, h NamedHandler, data any, cb func([]byte, error)) {
    m.lastMethod = method
    m.lastHandler = h.HandlerName()
    m.lastData = data
    if cb != nil {
        cb([]byte(`{"ok":true}`), nil)
    }
}

func TestDOMSend(t *testing.T) {
    sender := &mockSender{}
    dom := New(nil, sender)
    
    handler := &testHandler{}
    dom.Send(POST, handler, "test", nil)
    
    if sender.lastMethod != POST {
        t.Error("expected POST")
    }
}
```
