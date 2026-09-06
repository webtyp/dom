# `webtyp/dom` Design: Signals & Fine-Grained Reactivity

This document records the architectural decisions for the `webtyp/dom` reactive engine.

## 1. Rationale: Why Signals?

The previous model used a coarse `Update()` method that re-rendered the entire component subtree via `outerHTML`. This had several drawbacks:
- **Performance**: O(n) re-render per change.
- **Node Identity**: Re-rendering destroyed DOM nodes, breaking focus, text selection, and IME composition.
- **Complexity**: Authors had to manually call `Update()`, leading to "forgot to update" bugs.

### Alternatives Considered

| Approach | Pros | Cons |
|---|---|---|
| **Whole Re-render** | Simple to implement. | Destroys node identity; slow for large trees. |
| **Virtual DOM** | Preserves identity via diffing. | High memory overhead in TinyGo/WASM; complex implementation. |
| **Signals** | O(1) surgical patches; preserves identity. | Requires a specialized reactive engine. |

**Decision**: Signals were chosen because they provide the best performance and user experience (preserving focus/IME) while keeping the WASM binary size small compared to a full VDOM.

## 2. Typed Signals, No Generics

The ecosystem follows the `webtyp/fmt` codec rule: *"cero any, cero map"*. To stay consistent and minimize WASM overhead, we use concrete typed signals (`SignalString`, `SignalBool`, `SignalNodes`) instead of generics.

The DOM boundary is primarily `string` (attributes, text content) and `bool` (classes, boolean attributes), so these types cover 99% of use cases.

## 3. Auto-tracking over Explicit Dependencies

We use an internal `currentTracker` to automatically discover dependencies during signal `Get()` calls.

**Why?**
- **Intuitive**: Authors don't need to manually maintain dependency lists.
- **Correct by Construction**: It's impossible to forget a dependency.
- **Contained "Magic"**: While auto-tracking is reactive "magic", it's a well-understood pattern (SolidJS, Vue) that significantly improves developer ergonomics.

## 4. Default Escaping & Typed `TrustedHTML`

To prevent Stored and Reflected XSS vulnerabilities across applications using `webtyp/dom`, text nodes added via `.Text(string)` and reactive text bindings are escaped by default (`&`, `<`, `>`, `"`, `'`).

### Alternative Considered: `SafeText()`

An alternative design considered was keeping `.Text(string)` raw and introducing a separate `.SafeText(string)` method for escaped content.

**Why Discarded**:
- **Violates Principle 8 (Closed by default)**: "The safe default must be the one you get by writing nothing". If `.Text()` is unescaped by default, every developer oversight becomes an XSS vulnerability.
- **Unbounded Risk**: Requiring developers to explicitly call `SafeText()` means unsafe content is the default behavior.
- **Generic Type Slot**: Passing raw HTML strings as standard `string` loses developer intent.

**Decision**: `.Text(string)` escapes HTML always. Unescaped HTML injection requires `.Raw(dom.TrustedHTML)`, which can only be constructed via `dom.Trust(string)`. This guarantees that non-literal user input fails to compile when passed to `.Raw()`, and all exemptions from default escaping are easily auditable via `grep -rn "dom.Trust(" .`.

## 5. Construction Harness & Dev Diagnostics

To prevent silent failures and provide a better development experience:
- **Typed Builder**: Removing `Add(...any)` ensures that only valid types are used during element construction.
- **`devMode`**: A runtime flag that enables:
    - **Reactive Trace**: Logs which signal patched which node.
    - **Key Validation**: Warns on duplicate or empty keys in `BindChildren`.
    - **Harness Warnings**: Warns about nil signals or common component mistakes (e.g., pointer-embedded `Element`).
- **Nil-Safety**: Signal methods are nil-safe to prevent panics, turning a potential crash into a visible no-op with a dev warning.
