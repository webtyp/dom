package dom

// TrustedHTML is markup the program's AUTHOR guarantees is safe. The type
// exists so that putting untrusted data into the document fails to compile:
// there is no implicit conversion from string, and the only constructor
// forces a line a grep can find.
//
// Rule: only literals from the code itself, or the output of a builder from
// this ecosystem. NEVER a string that came from a request, a database, an
// OAuth profile, or another service.
type TrustedHTML string

// Trust marks html as trusted. It is the ONLY way to produce a TrustedHTML,
// and its name is what makes the program auditable: searching for
// "dom.Trust(" enumerates every point where escaping is deliberately
// skipped.
//
// If you are about to write dom.Trust(somethingThatCameFromOutside), the
// defect is in the caller's design, not here.
func Trust(html string) TrustedHTML { return TrustedHTML(html) }
