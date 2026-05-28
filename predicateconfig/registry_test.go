package predicateconfig

import (
	"testing"
)

// TestRegistryValidity iterates all registered predicates and checks that each
// entry is internally consistent. No predicate names are hard-coded — the test
// validates structural invariants across the whole registry.
func TestRegistryValidity(t *testing.T) {
	for id, c := range registry {
		// URIObject predicates must have AllowedOrigins set.
		if c.ValueShape == ValueShapeURIObject && c.AllowedOrigins == nil {
			t.Errorf("predicate %q: URIObject predicate must have AllowedOrigins set", id)
		}
		// Non-URIObject predicates must not have AllowedOrigins set.
		if c.ValueShape != ValueShapeURIObject && c.AllowedOrigins != nil {
			t.Errorf("predicate %q: non-URIObject predicate must not have AllowedOrigins set", id)
		}
		// Literal and URIObject predicates must have a PredicateURI.
		if (c.ValueShape == ValueShapeLiteral || c.ValueShape == ValueShapeURIObject) && c.PredicateURI == "" {
			t.Errorf("predicate %q: Literal/URIObject predicate must have a PredicateURI", id)
		}
		// ValueShapeOmit predicates may or may not have a PredicateURI:
		// - PredicateURI=="" → explicitly suppressed from RDF output (e.g. lastSuccessfulPlay)
		// - PredicateURI!="" → in registry for API reasons (e.g. MultiValue); RDF is handled by rdfgen switch
		// ResolveNameToURI and ResolveURIToName must be set together or both nil.
		if (c.ResolveNameToURI == nil) != (c.ResolveURIToName == nil) {
			t.Errorf("predicate %q: ResolveNameToURI and ResolveURIToName must both be set or both nil", id)
		}
		// Only URIObject predicates may have resolve functions.
		if c.ResolveNameToURI != nil && c.ValueShape != ValueShapeURIObject {
			t.Errorf("predicate %q: resolve functions only valid for URIObject predicates", id)
		}
		// LoganneSilent and LoganneHumanReadable are mutually exclusive.
		if c.LoganneSilent && c.LoganneHumanReadable != nil {
			t.Errorf("predicate %q: LoganneSilent predicate must not have LoganneHumanReadable set", id)
		}
		// Predicate ID must be non-empty (registry key sanity check).
		if id == "" {
			t.Error("registry contains an entry with an empty predicate ID")
		}
	}
}

// TestMultiValueCount checks the expected number of multi-value predicates.
func TestMultiValueCount(t *testing.T) {
	count := 0
	for _, c := range registry {
		if c.MultiValue {
			count++
		}
	}
	if count != 8 {
		t.Errorf("expected 8 multi-value predicates, got %d", count)
	}
}

// TestURIObjectCount checks the expected number of URIObject predicates.
func TestURIObjectCount(t *testing.T) {
	count := len(URIObjectPredicates())
	if count != 6 {
		t.Errorf("expected 6 URIObject predicates, got %d", count)
	}
}

// TestGetConfigUnknownReturnsZeroValue ensures GetConfig returns a zero-value Config
// for an unregistered predicate ID.
func TestGetConfigUnknownReturnsZeroValue(t *testing.T) {
	c := GetConfig("nonexistent_predicate_xyz")
	if c.MultiValue || c.ValueShape != ValueShapeOmit || c.PredicateURI != "" {
		t.Errorf("GetConfig for unknown predicate should return zero value, got: %+v", c)
	}
}

// TestGetKnownReturnsTrue verifies every entry in the registry is found by Get.
func TestGetKnownReturnsTrue(t *testing.T) {
	for id := range registry {
		if _, ok := Get(id); !ok {
			t.Errorf("Get(%q) returned ok=false for a known predicate", id)
		}
	}
}

// TestGetUnknownReturnsFalse verifies Get returns false for an unregistered predicate.
func TestGetUnknownReturnsFalse(t *testing.T) {
	if _, ok := Get("nonexistent_predicate_xyz"); ok {
		t.Error("Get for unknown predicate should return ok=false")
	}
}

// TestAllReturnsAllPredicates verifies All() returns the full registry without omissions.
func TestAllReturnsAllPredicates(t *testing.T) {
	all := All()
	if len(all) != len(registry) {
		t.Errorf("All() returned %d entries, expected %d", len(all), len(registry))
	}
	for id := range registry {
		if _, ok := all[id]; !ok {
			t.Errorf("All() is missing predicate %q", id)
		}
	}
}
