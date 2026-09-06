# FEAT_002: Form Validation

> **Status:** Planned  
> **Priority:** P1

## Overview

Add form handling with automatic validation. TinyDOM manages the debounce and validation flow, calling handler's validation methods.

**Note:** This is a simplified specification. Implementation details will be refined during development.

## Core Interfaces

```go
// interfaces.go

// FieldValidator validates individual fields (real-time UI feedback)
type FieldValidator interface {
    ValidateField(fieldName string, value string) error
}

// FormConfig configures form behavior
type FormConfig struct {
    DebounceMs       int    // Default: 200 (global config)
    ValidClass       string // CSS class for valid fields. Default: "valid"
    InvalidClass     string // CSS class for invalid fields. Default: "invalid"
    ErrorSuffix      string // Suffix for error elements. Default: "-error"
    ValidateOnChange bool   // Validate on input change. Default: true
    ValidateOnBlur   bool   // Validate on blur. Default: true
}
```

## Form Interface

```go
// form.go

// Form handles form validation and submission
type Form interface {
    // Bind connects form to validator
    Bind(formID string, validator FieldValidator) error
    
    // IsValid returns true if all fields pass validation
    IsValid() bool
    
    // OnValid called when all fields are valid
    OnValid(callback func(fields []FormField))
    
    // OnSubmit called on form submit
    OnSubmit(callback func(fields []FormField))
}

// FormField represents a form field (no maps)
type FormField struct {
    Name  string
    Value string
}
```

## Global Debounce Config

```go
// config.go

var globalConfig = struct {
    DebounceMs int
}{
    DebounceMs: 200, // Default 200ms
}

// SetDebounceMs sets global debounce time
func SetDebounceMs(ms int) {
    globalConfig.DebounceMs = ms
}
```

## DOM Interface Update

```go
// dom.go
type DOM interface {
    // ... existing methods
    
    // Form creates a form handler with optional config
    Form(cfg *FormConfig) Form
}
```

## Validation Flow

```
┌─────────────────────────────────────────────────────────────────┐
│  1. User types in input#name                                    │
│         ↓                                                       │
│  2. TinyDOM detects onChange                                    │
│         ↓                                                       │
│  3. Debounce (200ms default, configurable)                      │
│         ↓                                                       │
│  4. TinyDOM calls: validator.ValidateField("name", value)       │
│         ↓                                                       │
│  5a. If error:                                                  │
│      - Add "invalid" class to input                             │
│      - Set error text in #name-error                            │
│  5b. If valid:                                                  │
│      - Add "valid" class to input                               │
│      - Clear #name-error                                        │
│         ↓                                                       │
│  6. If ALL fields valid → call OnValid callback                 │
└─────────────────────────────────────────────────────────────────┘
```

## Usage Example

```go
// user_front.go
func (h *Handler) HandlerName() string { return "user" }

// ValidateField - used by webtyp/dom for real-time validation
func (h *Handler) ValidateField(fieldName, value string) error {
    switch fieldName {
    case "name":
        if value == "" {
            return tinystring.Err("name required")
        }
        if len(value) < 2 {
            return tinystring.Err("name too short")
        }
    case "email":
        if !strings.Contains(value, "@") {
            return tinystring.Err("invalid email")
        }
    }
    return nil
}

func (h *Handler) OnMount(dom webtyp/dom.DOM) {
    form := dom.Form(nil) // Use defaults
    form.Bind("user-form", h)
    
    form.OnValid(func(fields []webtyp/dom.FormField) {
        // All fields valid, can enable submit button
        btn, _ := dom.Get("submit-btn")
        btn.RemoveAttribute("disabled")
    })
    
    form.OnSubmit(func(fields []webtyp/dom.FormField) {
        // Convert fields to struct
        user := User{}
        for _, f := range fields {
            switch f.Name {
            case "name":
                user.Name = f.Value
            case "email":
                user.Email = f.Value
            }
        }
        
        // Send to server
        dom.Send(webtyp/dom.POST, h, user, h.onCreateResponse)
    })
}
```

## HTML Convention

```html
<form id="user-form">
    <input id="name" name="name" />
    <span id="name-error"></span>
    
    <input id="email" name="email" />
    <span id="email-error"></span>
    
    <button id="submit-btn" type="submit" disabled>Save</button>
</form>
```

## Files to Create/Modify

- `config.go`: Global debounce config
- `form.go`: Form interface and implementation
- `interfaces.go`: Add FieldValidator, FormConfig, FormField
- `dom.go`: Add Form() method

## Implementation Notes

- No maps used - FormField slice instead
- Debounce implemented without goroutines (use JS setTimeout in WASM)
- Error elements found by convention: `{fieldName}{ErrorSuffix}`
