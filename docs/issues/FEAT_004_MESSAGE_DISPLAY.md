# FEAT_004: Message Display

> **Status:** Planned  
> **Priority:** P2

## Overview

Add a method to display user notifications/messages. This is called by CRUDP broker when receiving responses.

## Interface

```go
// interfaces.go

// MessageDisplay shows messages to users
type MessageDisplay interface {
    ShowMessage(msgType uint8, message string)
}
```

**msgType values (from tinystring.MessageType):**
- `0` = Normal
- `1` = Info
- `2` = Error
- `3` = Warning
- `4` = Success

## DOM Interface Update

```go
// dom.go
type DOM interface {
    // ... existing methods
    
    // ShowMessage displays a notification to the user
    // msgType: tinystring.MessageType (0-4)
    ShowMessage(msgType uint8, message string)
}
```

## Implementation

```go
// dom_frontend.go
//go:build wasm

// MessageConfig configures message display
type MessageConfig struct {
    ContainerID string // Default: "messages"
    Duration    int    // Auto-hide after ms. Default: 3000. 0 = no auto-hide
}

var messageConfig = MessageConfig{
    ContainerID: "messages",
    Duration:    3000,
}

// SetMessageConfig sets message display configuration
func SetMessageConfig(cfg MessageConfig) {
    if cfg.ContainerID != "" {
        messageConfig.ContainerID = cfg.ContainerID
    }
    if cfg.Duration >= 0 {
        messageConfig.Duration = cfg.Duration
    }
}

func (d *domImpl) ShowMessage(msgType uint8, message string) {
    container, ok := d.Get(messageConfig.ContainerID)
    if !ok {
        d.Log("message container not found:", messageConfig.ContainerID)
        return
    }
    
    // Create message element
    msgClass := msgTypeToClass(msgType)
    msgID := "msg-" + generateID()
    
    html := `<div id="` + msgID + `" class="message ` + msgClass + `">` + message + `</div>`
    container.AppendHTML(html)
    
    // Auto-hide after duration
    if messageConfig.Duration > 0 {
        js.Global().Call("setTimeout", js.FuncOf(func(this js.Value, args []js.Value) any {
            if el, ok := d.Get(msgID); ok {
                el.Remove()
            }
            return nil
        }), messageConfig.Duration)
    }
}

func msgTypeToClass(t uint8) string {
    switch t {
    case 1:
        return "message-info"
    case 2:
        return "message-error"
    case 3:
        return "message-warning"
    case 4:
        return "message-success"
    default:
        return "message-normal"
    }
}
```

## Backend Stub

```go
// dom_backend.go
//go:build !wasm

func (d *domImpl) ShowMessage(msgType uint8, message string) {
    // Log in SSR context
    if d.log != nil {
        d.log("Message:", msgType, message)
    }
}
```

## HTML Setup

```html
<body>
    <!-- Message container -->
    <div id="messages"></div>
    
    <!-- App content -->
    <div id="app"></div>
</body>
```

## CSS Example

```css
.message {
    padding: 12px 20px;
    margin: 8px;
    border-radius: 4px;
    animation: fadeIn 0.3s;
}

.message-success { background: #d4edda; color: #155724; }
.message-error   { background: #f8d7da; color: #721c24; }
.message-warning { background: #fff3cd; color: #856404; }
.message-info    { background: #cce5ff; color: #004085; }
.message-normal  { background: #e2e3e5; color: #383d41; }

@keyframes fadeIn {
    from { opacity: 0; transform: translateY(-10px); }
    to   { opacity: 1; transform: translateY(0); }
}
```

## CRUDP Integration

CRUDP passes `ShowMessage` to its config:

```go
// main.go (wasm)
dom := webtyp/dom.New(log.Println, nil)

cfg := &crudp.Config{
    OnMessage: dom.ShowMessage,
}
cp := crudp.New(cfg)

// Update dom with sender
dom = webtyp/dom.New(log.Println, cp)
```

Or CRUDP can call it directly if DOM implements MessageDisplay:

```go
// broker_client.go
func (b *broker) onSSEMessage(data []byte) {
    var result PacketResult
    b.codec.Decode(data, &result)
    
    // Show message if DOM supports it
    if md, ok := b.dom.(webtyp/dom.MessageDisplay); ok {
        md.ShowMessage(result.MessageType, result.Message)
    }
    
    // ... continue processing
}
```

## Files to Modify

- `interfaces.go`: Add MessageDisplay interface
- `dom.go`: Add ShowMessage to DOM interface
- `dom_frontend.go`: Implement ShowMessage
- `dom_backend.go`: Add stub
- `config.go`: Add MessageConfig

## Notes

- Uses simple HTML append, no virtual DOM
- Auto-removes after configurable duration
- CSS classes for styling (user provides CSS)
