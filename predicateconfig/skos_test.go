package predicateconfig

import (
	"os"
	"strings"
	"testing"
)

// TestSKOSConceptsAllPredicatesHaveConcepts verifies all four SKOS predicates have concepts defined.
func TestSKOSConceptsAllPredicatesHaveConcepts(t *testing.T) {
	for _, predicate := range []string{"provenance", "availability", "singalong", "dance"} {
		concepts := GetSKOSConcepts(predicate)
		if len(concepts) == 0 {
			t.Errorf("predicate %q: expected at least one SKOS concept, got none", predicate)
		}
	}
}

// TestSKOSConceptsUnknownPredicateReturnsNil verifies that unknown predicates return nil.
func TestSKOSConceptsUnknownPredicateReturnsNil(t *testing.T) {
	if concepts := GetSKOSConcepts("nonexistent"); concepts != nil {
		t.Errorf("expected nil for unknown predicate, got %v", concepts)
	}
}

// TestSKOSConceptsNoDuplicateSlugs verifies that all slugs within each predicate are unique.
func TestSKOSConceptsNoDuplicateSlugs(t *testing.T) {
	for _, predicate := range []string{"provenance", "availability", "singalong", "dance"} {
		seen := make(map[string]bool)
		for _, c := range GetSKOSConcepts(predicate) {
			if seen[c.Slug] {
				t.Errorf("predicate %q: duplicate slug %q", predicate, c.Slug)
			}
			seen[c.Slug] = true
		}
	}
}

// TestSKOSConceptsNoDuplicatePrefLabels verifies that all prefLabels within each predicate are unique (case-insensitive).
func TestSKOSConceptsNoDuplicatePrefLabels(t *testing.T) {
	for _, predicate := range []string{"provenance", "availability", "singalong", "dance"} {
		seen := make(map[string]bool)
		for _, c := range GetSKOSConcepts(predicate) {
			lower := strings.ToLower(c.PrefLabel)
			if seen[lower] {
				t.Errorf("predicate %q: duplicate prefLabel %q", predicate, c.PrefLabel)
			}
			seen[lower] = true
		}
	}
}

// TestSKOSConceptsSlugIsKebabCase verifies that all slugs are non-empty and ASCII.
func TestSKOSConceptsSlugIsNonEmpty(t *testing.T) {
	for _, predicate := range []string{"provenance", "availability", "singalong", "dance"} {
		for _, c := range GetSKOSConcepts(predicate) {
			if c.Slug == "" {
				t.Errorf("predicate %q concept %q: slug must not be empty", predicate, c.PrefLabel)
			}
			if strings.Contains(c.Slug, " ") {
				t.Errorf("predicate %q concept %q: slug %q must not contain spaces", predicate, c.PrefLabel, c.Slug)
			}
		}
	}
}

// TestSKOSOrdinalLevelsAvailability verifies that availability has levels 0–4 in order.
func TestSKOSOrdinalLevelsAvailability(t *testing.T) {
	concepts := GetSKOSConcepts("availability")
	if len(concepts) != 5 {
		t.Fatalf("expected 5 availability concepts, got %d", len(concepts))
	}
	for i, c := range concepts {
		if c.Level != i {
			t.Errorf("availability concept[%d]: expected Level=%d, got Level=%d (slug=%q)", i, i, c.Level, c.Slug)
		}
	}
}

// TestSKOSOrdinalLevelsSingalong verifies that singalong has levels 0–5 in order.
func TestSKOSOrdinalLevelsSingalong(t *testing.T) {
	concepts := GetSKOSConcepts("singalong")
	if len(concepts) != 6 {
		t.Fatalf("expected 6 singalong concepts, got %d", len(concepts))
	}
	for i, c := range concepts {
		if c.Level != i {
			t.Errorf("singalong concept[%d]: expected Level=%d, got Level=%d (slug=%q)", i, i, c.Level, c.Slug)
		}
	}
}

// TestConceptURI verifies the URI construction pattern.
func TestConceptURI(t *testing.T) {
	uri := ConceptURI("https://media.l42.eu", "provenance", "bandcamp")
	expected := "https://media.l42.eu/vocab/provenance/bandcamp"
	if uri != expected {
		t.Errorf("expected %q, got %q", expected, uri)
	}
}

// TestResolveSlugToConceptURIBySlug verifies that slug-based lookup works.
func TestResolveSlugToConceptURIBySlug(t *testing.T) {
	uri, err := ResolveSlugToConceptURI("provenance", "https://media.l42.eu", "bandcamp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "https://media.l42.eu/vocab/provenance/bandcamp"
	if uri != expected {
		t.Errorf("expected %q, got %q", expected, uri)
	}
}

// TestResolveSlugToConceptURIByPrefLabel verifies that case-insensitive prefLabel fallback works.
func TestResolveSlugToConceptURIByPrefLabel(t *testing.T) {
	// "Lindy Hop" is the prefLabel; the slug is "lindy-hop"
	uri, err := ResolveSlugToConceptURI("dance", "https://media.l42.eu", "Lindy Hop")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "https://media.l42.eu/vocab/dance/lindy-hop"
	if uri != expected {
		t.Errorf("expected %q, got %q", expected, uri)
	}
}

// TestResolveSlugToConceptURICaseInsensitive verifies that case-insensitive lookup works.
func TestResolveSlugToConceptURICaseInsensitive(t *testing.T) {
	uri, err := ResolveSlugToConceptURI("dance", "https://media.l42.eu", "lindy hop")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "https://media.l42.eu/vocab/dance/lindy-hop"
	if uri != expected {
		t.Errorf("expected %q, got %q", expected, uri)
	}
}

// TestResolveSlugToConceptURIUnknownReturnsError verifies that unknown values return an error.
func TestResolveSlugToConceptURIUnknownReturnsError(t *testing.T) {
	_, err := ResolveSlugToConceptURI("provenance", "https://media.l42.eu", "nonexistent-value-xyz")
	if err == nil {
		t.Error("expected error for unknown value, got nil")
	}
}

// TestResolveConceptURIToNameRoundTrip verifies that slug → URI → name → slug round-trip works.
func TestResolveConceptURIToNameRoundTrip(t *testing.T) {
	appOrigin := "https://media.l42.eu"
	for _, predicate := range []string{"provenance", "availability", "singalong", "dance"} {
		for _, c := range GetSKOSConcepts(predicate) {
			uri := ConceptURI(appOrigin, predicate, c.Slug)
			// URI → name
			name, err := ResolveConceptURIToName(predicate, appOrigin, uri)
			if err != nil {
				t.Errorf("predicate %q slug %q: ResolveConceptURIToName error: %v", predicate, c.Slug, err)
				continue
			}
			if name != c.PrefLabel {
				t.Errorf("predicate %q slug %q: expected prefLabel %q, got %q", predicate, c.Slug, c.PrefLabel, name)
			}
			// name/slug → URI: try slug first (exact), then prefLabel fallback
			uri2, err := ResolveSlugToConceptURI(predicate, appOrigin, c.Slug)
			if err != nil {
				t.Errorf("predicate %q slug %q: ResolveSlugToConceptURI (by slug) error: %v", predicate, c.Slug, err)
				continue
			}
			if uri2 != uri {
				t.Errorf("predicate %q slug %q: round-trip URI mismatch: %q → %q → %q", predicate, c.Slug, uri, name, uri2)
			}
		}
	}
}

// TestSKOSResolveNameToURIViaEnv verifies the closure-based resolver reads APP_ORIGIN from env.
func TestSKOSResolveNameToURIViaEnv(t *testing.T) {
	os.Setenv("APP_ORIGIN", "https://media.l42.eu")
	t.Cleanup(func() { os.Unsetenv("APP_ORIGIN") })

	resolver := SKOSResolveNameToURI("provenance")
	uri, err := resolver(nil, "bandcamp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "https://media.l42.eu/vocab/provenance/bandcamp"
	if uri != expected {
		t.Errorf("expected %q, got %q", expected, uri)
	}
}

// TestSKOSResolveURIToNameViaEnv verifies the closure-based reverse resolver.
func TestSKOSResolveURIToNameViaEnv(t *testing.T) {
	os.Setenv("APP_ORIGIN", "https://media.l42.eu")
	t.Cleanup(func() { os.Unsetenv("APP_ORIGIN") })

	resolver := SKOSResolveURIToName("provenance")
	name, err := resolver(nil, "https://media.l42.eu/vocab/provenance/bandcamp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "Bandcamp" {
		t.Errorf("expected %q, got %q", "Bandcamp", name)
	}
}

// TestSKOSPredicatesHaveAllowedOrigins verifies that all SKOS predicates have AllowedOrigins set.
func TestSKOSPredicatesHaveAllowedOrigins(t *testing.T) {
	for _, predicate := range []string{"provenance", "availability", "singalong", "dance"} {
		c := GetConfig(predicate)
		if c.AllowedOrigins == nil {
			t.Errorf("predicate %q: AllowedOrigins must be set for URIObject predicates", predicate)
		}
		found := false
		for _, o := range c.AllowedOrigins {
			if o == OriginMediaMetadataAPI {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("predicate %q: AllowedOrigins must include OriginMediaMetadataAPI", predicate)
		}
	}
}

// TestValidateURIOriginAcceptsMediaMetadataAPIURI verifies that the new OriginMediaMetadataAPI
// origin constant is accepted when APP_ORIGIN matches the URI prefix.
func TestValidateURIOriginAcceptsMediaMetadataAPIURI(t *testing.T) {
	os.Setenv("APP_ORIGIN", "https://media.l42.eu")
	t.Cleanup(func() { os.Unsetenv("APP_ORIGIN") })
	c := Config{AllowedOrigins: []string{OriginMediaMetadataAPI}}
	uri := "https://media.l42.eu/vocab/provenance/bandcamp"
	if msg := c.ValidateURIOrigin(uri); msg != "" {
		t.Errorf("expected no error for valid media-api URI, got: %q", msg)
	}
}

// TestValidateURIOriginRejectsWrongOriginForMediaMetadataAPI verifies rejection of wrong-origin URIs.
func TestValidateURIOriginRejectsWrongOriginForMediaMetadataAPI(t *testing.T) {
	os.Setenv("APP_ORIGIN", "https://media.l42.eu")
	t.Cleanup(func() { os.Unsetenv("APP_ORIGIN") })
	c := Config{AllowedOrigins: []string{OriginMediaMetadataAPI}}
	uri := "https://eolas.l42.eu/vocab/provenance/bandcamp"
	if msg := c.ValidateURIOrigin(uri); msg == "" {
		t.Error("expected error for URI with wrong origin, got empty string")
	}
}

// TestGenreIsOmit verifies that genre is now ValueShapeOmit.
func TestGenreIsOmit(t *testing.T) {
	c := GetConfig("genre")
	if c.ValueShape != ValueShapeOmit {
		t.Errorf("expected genre to have ValueShapeOmit, got %v", c.ValueShape)
	}
	if c.PredicateURI != "" {
		t.Errorf("expected genre to have no PredicateURI (Omit), got %q", c.PredicateURI)
	}
}
