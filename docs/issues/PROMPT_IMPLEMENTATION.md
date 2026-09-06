# TinyDOM Implementation Prompt

> **For LLM:** Follow these instructions to implement TinyDOM features for CRUDP integration.

## Context

You are implementing CRUDP integration features for TinyDOM, a DOM manipulation library for isomorphic Go applications. The codebase uses:

- **TinyGo compatibility:** No maps in hot paths, minimal allocations
- **Build tags:** `//go:build wasm` for frontend, `//go:build !wasm` for backend
- **Error handling:** Use `webtyp.com/fmt` (Err, Errf) - already imported
- **Existing patterns:** Follow existing code style and patterns in element.go, dom.go

## Project Structure (Current + New Files)

```
webtyp/dom/
├── dom.go              # Core DOM shared code
├── dom_frontend.go     # WASM implementation (//go:build wasm)
├── dom_backend.go      # Server-side stubs (//go:build !wasm)
├── element.go          # Element interface definition
├── element_wasm.go     # WASM element implementation
├── event_wasm.go       # Event handling WASM
├── component_frontend.go
├── component_backend.go
├── webtyp/dom.go          # Package entry
├── go.mod              # Dependencies (tinystring v0.12.0)
│
│   # NEW FILES TO CREATE:
├── http_constants.go   # HTTP constants (shared)
├── sender.go           # Sender interface (shared)
├── sender_wasm.go      # WASM sender implementation
├── sender_backend.go   # Server-side sender stub
├── validation.go       # Validation helpers (shared)
├── validation_wasm.go  # Form validation WASM
├── debounce.go         # Debounce config (shared)
├── debounce_wasm.go    # Debounced events WASM
├── message.go          # Message types (shared)
└── message_wasm.go     # Message display WASM
```

## Implementation Order

Complete each task fully (code + tests) before moving to the next.

---

## Task 1: HTTP Constants and Sender Interface (FEAT_001)

### 1.1 Create HTTP constants

**File:** `http_constants.go` (create new)

```go
package webtyp/dom

// HTTP methods (string constants for fetchgo)
const (
    MethodGet    = "GET"
    MethodPost   = "POST"
    MethodPut    = "PUT"
    MethodDelete = "DELETE"
)

// CRUD actions (bytes for CRUDP)
const (
    ActionCreate = 'c'
    ActionRead   = 'r'
    ActionUpdate = 'u'
    ActionDelete = 'd'
)

// MethodToAction converts HTTP method to CRUD action
func MethodToAction(method string) byte {
    switch method {
    case MethodPost:
        return ActionCreate
    case MethodGet:
        return ActionRead
    case MethodPut:
        return ActionUpdate
    case MethodDelete:
        return ActionDelete
    default:
        return 0
    }
}

// ActionToMethod converts CRUD action to HTTP method
func ActionToMethod(action byte) string {
    switch action {
    case ActionCreate:
        return MethodPost
    case ActionRead:
        return MethodGet
    case ActionUpdate:
        return MethodPut
    case ActionDelete:
        return MethodDelete
    default:
        return ""
    }
}
```

### 1.2 Create Sender interface

**File:** `sender.go` (create new)

```go
package webtyp/dom

// SendRequest contains data for HTTP request
type SendRequest struct {
    Method      string            // HTTP method (GET, POST, PUT, DELETE)
    URL         string            // Request URL
    Body        []byte            // Request body (JSON/binary)
    ContentType string            // Content-Type header
    Headers     [][2]string       // Additional headers as key-value pairs
}

// SendResponse contains response data
type SendResponse struct {
    Status  int    // HTTP status code
    Body    []byte // Response body
    Error   error  // Network or parsing error
}

// Sender interface for HTTP communication
// Implemented by webtyp/dom (frontend) and injected into CRUDP
type Sender interface {
    // Send executes an HTTP request and returns the response
    Send(req SendRequest) SendResponse
}

// SenderAsync interface for non-blocking requests
type SenderAsync interface {
    Sender
    // SendAsync executes request and calls callback with response
    SendAsync(req SendRequest, callback func(SendResponse))
}

// EventSource interface for SSE connections
type EventSource interface {
    // Connect establishes SSE connection
    Connect(url string) error
    // OnMessage sets handler for incoming messages
    OnMessage(handler func(event, data string))
    // OnError sets handler for connection errors
    OnError(handler func(error))
    // Close terminates the SSE connection
    Close()
}
```

### 1.3 Create frontend sender implementation

**File:** `sender_wasm.go` (create new)

```go
//go:build wasm

package webtyp/dom

import (
    "webtyp.com/fetch"
)

// domSender implements Sender interface using fetchgo
type domSender struct {
    client *fetchgo.Client
}

// NewSender creates a new Sender for WASM environment
func NewSender() Sender {
    return &domSender{
        client: fetchgo.New(),
    }
}

// NewSenderAsync creates a new async Sender for WASM
func NewSenderAsync() SenderAsync {
    return &domSender{
        client: fetchgo.New(),
    }
}

func (s *domSender) Send(req SendRequest) SendResponse {
    resp := SendResponse{}
    
    // Build fetchgo request
    fresp, err := s.client.Request(req.Method, req.URL, req.Body)
    if err != nil {
        resp.Error = err
        return resp
    }
    
    resp.Status = fresp.Status
    resp.Body = fresp.Body
    return resp
}

func (s *domSender) SendAsync(req SendRequest, callback func(SendResponse)) {
    go func() {
        resp := s.Send(req)
        callback(resp)
    }()
}
```

### 1.4 Create backend sender stub

**File:** `sender_backend.go` (create new)

```go
//go:build !wasm

package webtyp/dom

import "webtyp.com/fmt"

// domSender stub for server-side (HTTP requests not needed on backend)
type domSender struct{}

// NewSender returns stub on backend (not needed)
func NewSender() Sender {
    return &domSender{}
}

// NewSenderAsync returns stub on backend (not needed)
func NewSenderAsync() SenderAsync {
    return &domSender{}
}

func (s *domSender) Send(req SendRequest) SendResponse {
    return SendResponse{
        Error: tinystring.Err("Send not available on backend"),
    }
}

func (s *domSender) SendAsync(req SendRequest, callback func(SendResponse)) {
    callback(SendResponse{
        Error: tinystring.Err("SendAsync not available on backend"),
    })
}
```

### 1.5 Tests

**File:** `sender_test.go` (create new)

```go
package webtyp/dom

import "testing"

func TestHTTPConstants(t *testing.T) {
    tests := []struct {
        method string
        action byte
    }{
        {MethodPost, ActionCreate},
        {MethodGet, ActionRead},
        {MethodPut, ActionUpdate},
        {MethodDelete, ActionDelete},
        {"INVALID", 0},
    }
    
    for _, tt := range tests {
        if got := MethodToAction(tt.method); got != tt.action {
            t.Errorf("MethodToAction(%s) = %c, want %c", tt.method, got, tt.action)
        }
        if tt.action != 0 {
            if got := ActionToMethod(tt.action); got != tt.method {
                t.Errorf("ActionToMethod(%c) = %s, want %s", tt.action, got, tt.method)
            }
        }
    }
}

func TestSendRequest(t *testing.T) {
    req := SendRequest{
        Method:      MethodPost,
        URL:         "/api",
        Body:        []byte(`{"test":true}`),
        ContentType: "application/json",
        Headers:     [][2]string{{"X-Custom", "value"}},
    }
    
    if req.Method != "POST" {
        t.Error("method mismatch")
    }
    if req.ContentType != "application/json" {
        t.Error("content-type mismatch")
    }
    if len(req.Headers) != 1 || req.Headers[0][0] != "X-Custom" {
        t.Error("headers mismatch")
    }
}
```

---

## Task 2: Form Validation (FEAT_002)

### 2.1 Create validation helpers

**File:** `validation.go` (create new)

```go
package webtyp/dom

import "webtyp.com/fmt"

// FieldError represents a single field validation error
type FieldError struct {
    Field   string
    Message string
}

// FieldValidator interface for field-level validation
// Compatible with CRUDP's FieldValidator
type FieldValidator interface {
    ValidateField(fieldName, value string) error
}

// FormValidator collects field validations
type FormValidator struct {
    errors []FieldError
}

// NewFormValidator creates a new form validator
func NewFormValidator() *FormValidator {
    return &FormValidator{
        errors: make([]FieldError, 0),
    }
}

// Validate validates a single field
func (v *FormValidator) Validate(validator FieldValidator, fieldName, value string) {
    if err := validator.ValidateField(fieldName, value); err != nil {
        v.errors = append(v.errors, FieldError{
            Field:   fieldName,
            Message: err.Error(),
        })
    }
}

// HasErrors returns true if there are validation errors
func (v *FormValidator) HasErrors() bool {
    return len(v.errors) > 0
}

// Errors returns all field errors
func (v *FormValidator) Errors() []FieldError {
    return v.errors
}

// FirstError returns the first error message
func (v *FormValidator) FirstError() string {
    if len(v.errors) > 0 {
        return v.errors[0].Message
    }
    return ""
}

// Clear removes all validation errors
func (v *FormValidator) Clear() {
    v.errors = v.errors[:0]
}
```

### 2.2 Add form validation to Element (frontend only)

**File:** `element_validation_wasm.go` (create new)

```go
//go:build wasm

package webtyp/dom

import (
    "syscall/js"
)

// ValidateInput validates an input element using FieldValidator
func (e *Element) ValidateInput(validator FieldValidator) error {
    if e.el.IsUndefined() || e.el.IsNull() {
        return Err("element is nil")
    }
    
    name := e.el.Get("name").String()
    value := e.el.Get("value").String()
    
    return validator.ValidateField(name, value)
}

// ValidateForm validates all inputs in a form
func (e *Element) ValidateForm(validator FieldValidator) *FormValidator {
    fv := NewFormValidator()
    
    if e.el.IsUndefined() || e.el.IsNull() {
        return fv
    }
    
    // Get all input elements
    inputs := e.el.Call("querySelectorAll", "input, select, textarea")
    length := inputs.Get("length").Int()
    
    for i := 0; i < length; i++ {
        input := inputs.Index(i)
        name := input.Get("name").String()
        if name == "" {
            continue
        }
        value := input.Get("value").String()
        fv.Validate(validator, name, value)
    }
    
    return fv
}

// ShowFieldError displays validation error on field
func (e *Element) ShowFieldError(fieldName, message string) {
    if e.el.IsUndefined() || e.el.IsNull() {
        return
    }
    
    // Find the field
    field := e.el.Call("querySelector", "[name='"+fieldName+"']")
    if field.IsNull() || field.IsUndefined() {
        return
    }
    
    // Add error class
    field.Get("classList").Call("add", "error")
    
    // Find or create error message element
    parent := field.Get("parentElement")
    if parent.IsNull() || parent.IsUndefined() {
        return
    }
    
    errorEl := parent.Call("querySelector", ".field-error")
    if errorEl.IsNull() || errorEl.IsUndefined() {
        errorEl = js.Global().Get("document").Call("createElement", "span")
        errorEl.Get("classList").Call("add", "field-error")
        parent.Call("appendChild", errorEl)
    }
    
    errorEl.Set("textContent", message)
}

// ClearFieldError removes validation error from field
func (e *Element) ClearFieldError(fieldName string) {
    if e.el.IsUndefined() || e.el.IsNull() {
        return
    }
    
    field := e.el.Call("querySelector", "[name='"+fieldName+"']")
    if field.IsNull() || field.IsUndefined() {
        return
    }
    
    field.Get("classList").Call("remove", "error")
    
    parent := field.Get("parentElement")
    if parent.IsNull() || parent.IsUndefined() {
        return
    }
    
    errorEl := parent.Call("querySelector", ".field-error")
    if !errorEl.IsNull() && !errorEl.IsUndefined() {
        errorEl.Call("remove")
    }
}

// ClearAllErrors removes all validation errors from form
func (e *Element) ClearAllErrors() {
    if e.el.IsUndefined() || e.el.IsNull() {
        return
    }
    
    // Remove error class from all fields
    errorFields := e.el.Call("querySelectorAll", ".error")
    length := errorFields.Get("length").Int()
    for i := 0; i < length; i++ {
        errorFields.Index(i).Get("classList").Call("remove", "error")
    }
    
    // Remove all error messages
    errorMsgs := e.el.Call("querySelectorAll", ".field-error")
    length = errorMsgs.Get("length").Int()
    for i := 0; i < length; i++ {
        errorMsgs.Index(i).Call("remove")
    }
}
```

### 2.3 Tests

**File:** `validation_test.go` (create new)

```go
package webtyp/dom

import (
    "testing"
    
    "webtyp.com/fmt"
)

type testValidator struct{}

func (v *testValidator) ValidateField(fieldName, value string) error {
    switch fieldName {
    case "name":
        if value == "" {
            return tinystring.Err("name is required")
        }
    case "email":
        if value == "" {
            return tinystring.Err("email is required")
        }
        // Simple email check
        hasAt := false
        for _, c := range value {
            if c == '@' {
                hasAt = true
                break
            }
        }
        if !hasAt {
            return tinystring.Err("invalid email format")
        }
    case "age":
        if value == "" {
            return tinystring.Err("age is required")
        }
    }
    return nil
}

func TestFormValidator(t *testing.T) {
    validator := &testValidator{}
    fv := NewFormValidator()
    
    // Valid field
    fv.Validate(validator, "name", "John")
    if fv.HasErrors() {
        t.Error("expected no errors for valid name")
    }
    
    // Invalid field
    fv.Validate(validator, "email", "invalid")
    if !fv.HasErrors() {
        t.Error("expected error for invalid email")
    }
    
    if fv.FirstError() != "invalid email format" {
        t.Errorf("expected 'invalid email format', got %s", fv.FirstError())
    }
}

func TestFormValidator_Multiple(t *testing.T) {
    validator := &testValidator{}
    fv := NewFormValidator()
    
    fv.Validate(validator, "name", "")
    fv.Validate(validator, "email", "")
    fv.Validate(validator, "age", "")
    
    errors := fv.Errors()
    if len(errors) != 3 {
        t.Errorf("expected 3 errors, got %d", len(errors))
    }
}

func TestFormValidator_Clear(t *testing.T) {
    validator := &testValidator{}
    fv := NewFormValidator()
    
    fv.Validate(validator, "name", "")
    fv.Clear()
    
    if fv.HasErrors() {
        t.Error("expected no errors after clear")
    }
}
```

---

## Task 3: Debounce Configuration (FEAT_003)

### 3.1 Create debounce config

**File:** `debounce.go` (create new)

```go
package webtyp/dom

// DebounceConfig configures debounce behavior
type DebounceConfig struct {
    // Wait time in milliseconds before triggering
    Wait int
    
    // Immediate triggers on leading edge
    Immediate bool
    
    // MaxWait is maximum time to wait (0 = unlimited)
    MaxWait int
}

// DefaultDebounce returns default debounce configuration
func DefaultDebounce() DebounceConfig {
    return DebounceConfig{
        Wait:      300,  // 300ms default
        Immediate: false,
        MaxWait:   0,
    }
}

// ValidationDebounce returns debounce config for input validation
func ValidationDebounce() DebounceConfig {
    return DebounceConfig{
        Wait:      150,  // Faster for field validation
        Immediate: false,
        MaxWait:   500,
    }
}

// SearchDebounce returns debounce config for search inputs
func SearchDebounce() DebounceConfig {
    return DebounceConfig{
        Wait:      300,
        Immediate: false,
        MaxWait:   1000,
    }
}

// SubmitDebounce returns debounce config for form submission
func SubmitDebounce() DebounceConfig {
    return DebounceConfig{
        Wait:      500,  // Prevent double-submit
        Immediate: true, // Submit immediately, then debounce
        MaxWait:   0,
    }
}
```

### 3.2 Add debounced events (frontend only)

**File:** `debounce_wasm.go` (create new)

```go
//go:build wasm

package webtyp/dom

import (
    "syscall/js"
    "time"
)

// debouncer manages debounced function calls
type debouncer struct {
    timer    *time.Timer
    config   DebounceConfig
    lastCall int64
}

func newDebouncer(config DebounceConfig) *debouncer {
    return &debouncer{
        config: config,
    }
}

// Call executes the function with debouncing
func (d *debouncer) Call(fn func()) {
    now := time.Now().UnixMilli()
    
    // Immediate mode: call on first trigger
    if d.config.Immediate && d.timer == nil {
        fn()
        d.lastCall = now
    }
    
    // Cancel existing timer
    if d.timer != nil {
        d.timer.Stop()
    }
    
    // Check max wait
    if d.config.MaxWait > 0 && d.lastCall > 0 {
        elapsed := now - d.lastCall
        if elapsed >= int64(d.config.MaxWait) {
            fn()
            d.lastCall = now
            d.timer = nil
            return
        }
    }
    
    // Set new timer
    d.timer = time.AfterFunc(time.Duration(d.config.Wait)*time.Millisecond, func() {
        if !d.config.Immediate {
            fn()
        }
        d.lastCall = time.Now().UnixMilli()
        d.timer = nil
    })
}

// OnInputDebounced adds debounced input event listener
func (e *Element) OnInputDebounced(config DebounceConfig, handler func(value string)) {
    if e.el.IsUndefined() || e.el.IsNull() {
        return
    }
    
    d := newDebouncer(config)
    
    callback := js.FuncOf(func(this js.Value, args []js.Value) any {
        value := e.el.Get("value").String()
        d.Call(func() {
            handler(value)
        })
        return nil
    })
    
    e.el.Call("addEventListener", "input", callback)
}

// OnClickDebounced adds debounced click event listener
func (e *Element) OnClickDebounced(config DebounceConfig, handler func()) {
    if e.el.IsUndefined() || e.el.IsNull() {
        return
    }
    
    d := newDebouncer(config)
    
    callback := js.FuncOf(func(this js.Value, args []js.Value) any {
        d.Call(handler)
        return nil
    })
    
    e.el.Call("addEventListener", "click", callback)
}

// OnSubmitDebounced adds debounced submit event listener
func (e *Element) OnSubmitDebounced(config DebounceConfig, handler func()) {
    if e.el.IsUndefined() || e.el.IsNull() {
        return
    }
    
    d := newDebouncer(config)
    
    callback := js.FuncOf(func(this js.Value, args []js.Value) any {
        if len(args) > 0 {
            args[0].Call("preventDefault")
        }
        d.Call(handler)
        return nil
    })
    
    e.el.Call("addEventListener", "submit", callback)
}
```

### 3.3 Tests

**File:** `debounce_test.go` (create new)

```go
package webtyp/dom

import "testing"

func TestDebounceConfigs(t *testing.T) {
    def := DefaultDebounce()
    if def.Wait != 300 {
        t.Errorf("DefaultDebounce.Wait = %d, want 300", def.Wait)
    }
    
    val := ValidationDebounce()
    if val.Wait != 150 {
        t.Errorf("ValidationDebounce.Wait = %d, want 150", val.Wait)
    }
    
    search := SearchDebounce()
    if search.MaxWait != 1000 {
        t.Errorf("SearchDebounce.MaxWait = %d, want 1000", search.MaxWait)
    }
    
    submit := SubmitDebounce()
    if !submit.Immediate {
        t.Error("SubmitDebounce.Immediate should be true")
    }
}

func TestDebounceConfig_Values(t *testing.T) {
    config := DebounceConfig{
        Wait:      500,
        Immediate: true,
        MaxWait:   2000,
    }
    
    if config.Wait != 500 {
        t.Error("Wait not set correctly")
    }
    if !config.Immediate {
        t.Error("Immediate not set correctly")
    }
    if config.MaxWait != 2000 {
        t.Error("MaxWait not set correctly")
    }
}
```

---

## Task 4: Message Display (FEAT_004)

### 4.1 Create message types and display

**File:** `message.go` (create new)

```go
package webtyp/dom

// Message types align with tinystring.MessageType values:
// 0=Normal, 1=Info, 2=Error, 3=Warning, 4=Success
const (
    MsgNormal  uint8 = 0
    MsgInfo    uint8 = 1
    MsgError   uint8 = 2
    MsgWarning uint8 = 3
    MsgSuccess uint8 = 4
)

// Message represents a user-facing message
type Message struct {
    Type    uint8  // MessageType from tinystring
    Text    string // Message text
    Timeout int    // Auto-dismiss timeout in ms (0 = manual dismiss)
}

// NewMessage creates a new message
func NewMessage(msgType uint8, text string) Message {
    return Message{
        Type: msgType,
        Text: text,
    }
}

// WithTimeout sets auto-dismiss timeout
func (m Message) WithTimeout(ms int) Message {
    m.Timeout = ms
    return m
}

// MessageTypeClass returns CSS class for message type
func MessageTypeClass(msgType uint8) string {
    switch msgType {
    case MsgNormal:
        return "msg-normal"
    case MsgInfo:
        return "msg-info"
    case MsgError:
        return "msg-error"
    case MsgWarning:
        return "msg-warning"
    case MsgSuccess:
        return "msg-success"
    default:
        return "msg-normal"
    }
}
```

### 4.2 Message display (frontend only)

**File:** `message_wasm.go` (create new)

```go
//go:build wasm

package webtyp/dom

import (
    "syscall/js"
    "time"
)

// MessageDisplay manages user-facing messages
type MessageDisplay struct {
    container js.Value
}

// NewMessageDisplay creates a message display attached to container element
func NewMessageDisplay(containerID string) *MessageDisplay {
    container := js.Global().Get("document").Call("getElementById", containerID)
    return &MessageDisplay{
        container: container,
    }
}

// Show displays a message
func (md *MessageDisplay) Show(msg Message) {
    if md.container.IsUndefined() || md.container.IsNull() {
        return
    }
    
    doc := js.Global().Get("document")
    msgEl := doc.Call("createElement", "div")
    
    // Set classes
    msgEl.Get("classList").Call("add", "message", MessageTypeClass(msg.Type))
    
    // Set content
    msgEl.Set("textContent", msg.Text)
    
    // Add close button
    closeBtn := doc.Call("createElement", "button")
    closeBtn.Get("classList").Call("add", "msg-close")
    closeBtn.Set("innerHTML", "&times;")
    closeBtn.Call("addEventListener", "click", js.FuncOf(func(this js.Value, args []js.Value) any {
        msgEl.Call("remove")
        return nil
    }))
    msgEl.Call("appendChild", closeBtn)
    
    // Add to container
    md.container.Call("appendChild", msgEl)
    
    // Auto-dismiss
    if msg.Timeout > 0 {
        go func() {
            time.Sleep(time.Duration(msg.Timeout) * time.Millisecond)
            if !msgEl.IsNull() && !msgEl.IsUndefined() {
                msgEl.Call("remove")
            }
        }()
    }
}

// ShowError shows an error message
func (md *MessageDisplay) ShowError(text string) {
    md.Show(NewMessage(2, text).WithTimeout(5000))
}

// ShowSuccess shows a success message
func (md *MessageDisplay) ShowSuccess(text string) {
    md.Show(NewMessage(4, text).WithTimeout(3000))
}

// ShowInfo shows an info message
func (md *MessageDisplay) ShowInfo(text string) {
    md.Show(NewMessage(1, text).WithTimeout(4000))
}

// ShowWarning shows a warning message
func (md *MessageDisplay) ShowWarning(text string) {
    md.Show(NewMessage(3, text).WithTimeout(4000))
}

// Clear removes all messages
func (md *MessageDisplay) Clear() {
    if md.container.IsUndefined() || md.container.IsNull() {
        return
    }
    md.container.Set("innerHTML", "")
}
```

### 4.3 Message display stub (backend)

**File:** `message_backend.go` (create new)

```go
//go:build !wasm

package webtyp/dom

// MessageDisplay stub for backend
type MessageDisplay struct{}

// NewMessageDisplay returns nil on backend
func NewMessageDisplay(containerID string) *MessageDisplay {
    return &MessageDisplay{}
}

func (md *MessageDisplay) Show(msg Message)         {}
func (md *MessageDisplay) ShowError(text string)    {}
func (md *MessageDisplay) ShowSuccess(text string)  {}
func (md *MessageDisplay) ShowInfo(text string)     {}
func (md *MessageDisplay) ShowWarning(text string)  {}
func (md *MessageDisplay) Clear()                   {}
```

### 4.4 Tests

**File:** `message_test.go` (create new)

```go
package webtyp/dom

import "testing"

func TestMessage(t *testing.T) {
    msg := NewMessage(MsgError, "Error occurred")
    
    if msg.Type != MsgError {
        t.Errorf("Type = %d, want %d", msg.Type, MsgError)
    }
    if msg.Text != "Error occurred" {
        t.Errorf("Text = %s, want 'Error occurred'", msg.Text)
    }
    if msg.Timeout != 0 {
        t.Errorf("Timeout = %d, want 0", msg.Timeout)
    }
}

func TestMessage_WithTimeout(t *testing.T) {
    msg := NewMessage(MsgSuccess, "Success").WithTimeout(3000)
    
    if msg.Timeout != 3000 {
        t.Errorf("Timeout = %d, want 3000", msg.Timeout)
    }
}

func TestMessageTypeClass(t *testing.T) {
    tests := []struct {
        msgType uint8
        want    string
    }{
        {MsgNormal, "msg-normal"},
        {MsgInfo, "msg-info"},
        {MsgError, "msg-error"},
        {MsgWarning, "msg-warning"},
        {MsgSuccess, "msg-success"},
        {99, "msg-normal"}, // Unknown defaults to normal
    }
    
    for _, tt := range tests {
        if got := MessageTypeClass(tt.msgType); got != tt.want {
            t.Errorf("MessageTypeClass(%d) = %s, want %s", tt.msgType, got, tt.want)
        }
    }
}

func TestMessageConstants(t *testing.T) {
    // Verify constants match tinystring.MessageType values
    if MsgNormal != 0 {
        t.Error("MsgNormal should be 0")
    }
    if MsgInfo != 1 {
        t.Error("MsgInfo should be 1")
    }
    if MsgError != 2 {
        t.Error("MsgError should be 2")
    }
    if MsgWarning != 3 {
        t.Error("MsgWarning should be 3")
    }
    if MsgSuccess != 4 {
        t.Error("MsgSuccess should be 4")
    }
}
```

---

## Verification Checklist

- [ ] All tests pass: `go test ./...`
- [ ] No compilation errors
- [ ] WASM build works: `GOOS=js GOARCH=wasm go build ./...`
- [ ] TinyGo build works: `tinygo build -target=wasm ./...`
- [ ] Sender interface compatible with fetchgo
- [ ] HTTP constants work for both webtyp/dom and CRUDP
- [ ] Form validation is reusable with CRUDP handlers
- [ ] Debounce configs are sensible defaults
- [ ] Message display supports all MessageType values
- [ ] No fmt package used (use tinystring)

---

## Notes for LLM

1. **Import tinystring correctly (webtyp/dom style):**
   ```go
   import "webtyp.com/fmt"
   // Then use: tinystring.Err(), tinystring.Errf()
   ```
   
   NOT dot import like CRUDP uses.

2. **Build tags are critical:**
   - WASM code: `//go:build wasm` at file start
   - Backend stubs: `//go:build !wasm` at file start

3. **Check existing patterns** in the codebase:
   - See `dom_backend.go` for backend stub pattern
   - See `element_wasm.go` for WASM implementation pattern
   - See `element.go` for interface definitions

4. **Run both builds:**
   ```bash
   go test ./...
   GOOS=js GOARCH=wasm go build ./...
   ```

5. **Integration with CRUDP:**
   - Sender interface is injected into CRUDP for HTTP requests
   - FieldValidator interface is shared between both packages
   - MessageType values are consistent with tinystring

6. **Preserve existing functionality** when updating files

7. **File naming convention:**
   - `*_wasm.go` for WASM-only code (with `//go:build wasm`)
   - `*_backend.go` for backend stubs (with `//go:build !wasm`)
   - Base file (no suffix) for shared code/interfaces
