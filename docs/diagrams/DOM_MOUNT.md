# DOM Mount Without `innerHTML`

Context: Under strict `Content-Security-Policy` (`require-trusted-types-for 'script'` and `style-src 'self'`), `dom` builds live DOM nodes directly instead of parsing HTML strings via `innerHTML` or `insertAdjacentHTML`, and applies dynamic inline CSS properties through CSSOM (`element.style.setProperty`).

```mermaid
flowchart TD
    Start["Render / Append / Reconcile"] --> HasRoot{"Has *Element root?"}
    HasRoot -- Yes --> Pass["beginPass() / buildNode()"]
    Pass --> TagNS{"tag == 'svg' or in SVG subtree?"}
    TagNS -- Yes --> CreateSVG["document.createElementNS(SVG_NS, tag)"]
    TagNS -- No --> CreateHTML["document.createElement(tag)"]
    CreateSVG --> ApplyAttrs
    CreateHTML --> ApplyAttrs["Apply id, classes, data-key, attrs, StyleProp (CSSOM)"]
    ApplyAttrs --> Children["Append text nodes, *Element children, or Component children"]
    Children --> Insert["replaceChildren / appendChild / insertBefore"]
    Insert --> Wire["wirePendingEvents() + wireBindings() + Mounted()"]

    HasRoot -- No (legacy Component.String) --> TT{"window.trustedTypes exists?"}
    TT -- Yes --> Policy["Single policy 'webtyp-dom' -> createHTML()"]
    TT -- No --> Direct["Raw string assignment"]
    Policy --> InsertLegacy["Set innerHTML / <template> fallback"]
    Direct --> InsertLegacy
    InsertLegacy --> Wire
```

## Walkthrough

1. `Render` clears the mount target with `parent.replaceChildren()` instead of setting `innerHTML = ""`.
2. When the component produces an `*Element` tree, `buildNode` constructs each node with `createElement` or `createElementNS("http://www.w3.org/2000/svg", tag)` for `<svg>` subtrees.
3. Attributes, classes, `data-key`, and initial signal-bound values are written with DOM APIs (`setAttribute`, `classList`), and `Element.StyleProp` calls `node.style.setProperty(prop, val)` without emitting any `style="..."` HTML attribute.
4. Child `*Element` nodes and text nodes (`createTextNode`) are appended recursively; `TrustedHTML` fragments or legacy string-only `Component`s pass through a single cached `trustedTypes.createPolicy("webtyp-dom", ...)` helper.
5. Finally, `wirePendingEvents`, `wireBindings`, and `Mounted` run identically to the previous mount lifecycle.
