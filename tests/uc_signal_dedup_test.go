package dom_test

import (
	"testing"
	"webtyp.com/dom"
)

func TestDeriveString_DeduplicatesReads(t *testing.T) {
	s := dom.NewString("a")
	count := 0

	// Read the same signal twice
	derived := dom.DeriveString(func() string {
		count++
		return s.Get() + s.Get()
	})

	if derived.Get() != "aa" {
		t.Fatalf("expected 'aa', got %q", derived.Get())
	}
	if count != 1 {
		t.Fatalf("expected compute count 1, got %d", count)
	}

	// Set the signal once
	s.Set("b")

	if derived.Get() != "bb" {
		t.Fatalf("expected 'bb', got %q", derived.Get())
	}
	// The tracker should have deduplicated the double read in the compute function,
	// meaning there should only be ONE subscription to `s`.
	// Therefore, setting `s` should only run the updater ONCE.
	if count != 2 {
		t.Fatalf("expected compute count 2 (deduplicated), got %d — tracker is not deduplicating signal subscriptions", count)
	}
}
