package predicateconfig

// Package-level SKOS concept definitions for provenance, availability, singalong, and dance.
//
// Concept URIs follow the pattern: {APP_ORIGIN}/vocab/{predicate}/{slug}.
// For ordinal predicates (availability, singalong) the slug is the integer level number.
// For categorical predicates (provenance, dance) the slug is kebab-case ASCII derived from the prefLabel.

import (
	"fmt"
	"os"
	"strings"
)

// SKOSConcept represents a single concept in a SKOS concept scheme.
type SKOSConcept struct {
	Slug      string // canonical kebab-case ASCII identifier
	PrefLabel string // human-readable English label
	Level     int    // ordinal level, meaningful only for ordinal predicates (availability, singalong)
}

// provenanceConcepts defines all concepts in the provenance SKOS scheme.
// Slugs are sourced from lucos_media_metadata_manager/src/formfields.php (the "provenance" keys).
// PrefLabels are the corresponding display names.
var provenanceConcepts = []SKOSConcept{
	{Slug: "7digital", PrefLabel: "7 Digital"},
	{Slug: "amazon", PrefLabel: "Amazon Music"},
	{Slug: "ambiguous", PrefLabel: "Ambiguous"},
	{Slug: "audio-software", PrefLabel: "Produced using audio software"},
	{Slug: "bandcamp", PrefLabel: "Bandcamp"},
	{Slug: "cd-rip", PrefLabel: "CD Rip"},
	{Slug: "charity-auction", PrefLabel: "Charity Auction"},
	{Slug: "download", PrefLabel: "Download"},
	{Slug: "game", PrefLabel: "Game Soundtrack"},
	{Slug: "itunes", PrefLabel: "iTunes"},
	{Slug: "live", PrefLabel: "Live recording"},
	{Slug: "newgrounds", PrefLabel: "Newgrounds"},
	{Slug: "qobuz", PrefLabel: "Qobuz"},
	{Slug: "radio", PrefLabel: "Recorded from Radio"},
	{Slug: "sample", PrefLabel: "Free Sample"},
	{Slug: "vinyl", PrefLabel: "Recorded from a vinyl record"},
}

// availabilityConcepts defines all concepts in the availability SKOS scheme.
// The Level field is the ordinal value (0 = hardest to replace / most unique, 4 = easiest to replace / ubiquitous).
// Legacy DB rows store the integer level as a string (e.g. "0", "1", "2"...).
// Slugs are the integer level numbers; URIs follow the pattern {APP_ORIGIN}/vocab/availability/{level}.
var availabilityConcepts = []SKOSConcept{
	{Slug: "0", PrefLabel: "I have the canonical copy", Level: 0},
	{Slug: "1", PrefLabel: "Likely can't find elsewhere", Level: 1},
	{Slug: "2", PrefLabel: "Would need research to find", Level: 2},
	{Slug: "3", PrefLabel: "Could find after nontrivial searching", Level: 3},
	{Slug: "4", PrefLabel: "Ubiquitous", Level: 4},
}

// singalongConcepts defines all concepts in the singalong SKOS scheme.
// The Level field is the ordinal value (0 = can't sing along, 5 = fully a cappella).
// Legacy DB rows store the integer level as a string (e.g. "0", "1", "2"...).
// Slugs are the integer level numbers; URIs follow the pattern {APP_ORIGIN}/vocab/singalong/{level}.
var singalongConcepts = []SKOSConcept{
	{Slug: "0", PrefLabel: "No chance", Level: 0},
	{Slug: "1", PrefLabel: "Hum a Bit", Level: 1},
	{Slug: "2", PrefLabel: "Join in with the chorus in a club", Level: 2},
	{Slug: "3", PrefLabel: "Give it a go at karaoke", Level: 3},
	{Slug: "4", PrefLabel: "Karaoke without looking at the screen", Level: 4},
	{Slug: "5", PrefLabel: "Do it accapella without lyric sheet", Level: 5},
}

// danceConcepts defines all concepts in the dance SKOS scheme.
// Slugs are kebab-case ASCII derived from the prefLabels.
// Legacy DB rows store the display name as the value (e.g. "Lindy Hop", "Charleston").
// The "bespoke" concept is a special case: its slug and prefLabel differ.
var danceConcepts = []SKOSConcept{
	{Slug: "lindy-hop", PrefLabel: "Lindy Hop"},
	{Slug: "charleston", PrefLabel: "Charleston"},
	{Slug: "blues", PrefLabel: "Blues"},
	{Slug: "collegiate-shag", PrefLabel: "Collegiate Shag"},
	{Slug: "ceili", PrefLabel: "Céilí"},
	{Slug: "irish-step", PrefLabel: "Irish Step"},
	{Slug: "ceilidh", PrefLabel: "Ceilidh"},
	{Slug: "morris", PrefLabel: "Morris"},
	{Slug: "regency", PrefLabel: "Regency"},
	{Slug: "waltz", PrefLabel: "Waltz"},
	{Slug: "foxtrot", PrefLabel: "Foxtrot"},
	{Slug: "tango", PrefLabel: "Tango"},
	{Slug: "quickstep", PrefLabel: "Quickstep"},
	{Slug: "rhumba", PrefLabel: "Rhumba"},
	{Slug: "samba", PrefLabel: "Samba"},
	{Slug: "cha-cha", PrefLabel: "Cha Cha"},
	{Slug: "jive", PrefLabel: "Jive"},
	{Slug: "rock-n-roll", PrefLabel: "Rock'n'Roll"},
	{Slug: "disco", PrefLabel: "Disco"},
	{Slug: "bhangra", PrefLabel: "Bhangra"},
	{Slug: "bossa-nova", PrefLabel: "Bossa Nova"},
	{Slug: "mambo", PrefLabel: "Mambo"},
	{Slug: "bespoke", PrefLabel: "Track has its own dance"},
}

// conceptsByPredicate maps each predicate name to its concept list.
var conceptsByPredicate = map[string][]SKOSConcept{
	"provenance":   provenanceConcepts,
	"availability": availabilityConcepts,
	"singalong":    singalongConcepts,
	"dance":        danceConcepts,
}

// GetSKOSConcepts returns the concept list for the given predicate, or nil if not a SKOS predicate.
func GetSKOSConcepts(predicate string) []SKOSConcept {
	return conceptsByPredicate[predicate]
}

// ConceptURI returns the full URI for a concept given the APP_ORIGIN, predicate, and slug.
func ConceptURI(appOrigin, predicate, slug string) string {
	return fmt.Sprintf("%s/vocab/%s/%s", appOrigin, predicate, slug)
}

// ResolveSlugToConceptURI resolves a slug (or case-insensitive prefLabel) to its concept URI
// for the given predicate. Returns an error if no matching concept is found.
//
// Lookup order:
//  1. Exact slug match.
//  2. Case-insensitive prefLabel match (fallback for legacy values stored as display names).
func ResolveSlugToConceptURI(predicate, appOrigin, nameOrSlug string) (string, error) {
	concepts, ok := conceptsByPredicate[predicate]
	if !ok {
		return "", fmt.Errorf("predicate %q is not a SKOS concept scheme predicate", predicate)
	}

	appOrigin = strings.TrimSuffix(appOrigin, "/")

	// 1. Exact slug match.
	for _, c := range concepts {
		if c.Slug == nameOrSlug {
			return ConceptURI(appOrigin, predicate, c.Slug), nil
		}
	}

	// 2. Case-insensitive prefLabel match.
	lower := strings.ToLower(nameOrSlug)
	for _, c := range concepts {
		if strings.ToLower(c.PrefLabel) == lower {
			return ConceptURI(appOrigin, predicate, c.Slug), nil
		}
	}

	return "", fmt.Errorf("no concept found for predicate %q with name or slug %q", predicate, nameOrSlug)
}

// ResolveConceptURIToName resolves a concept URI back to its prefLabel (the human-readable name
// stored in the tag's value column). Returns an error if the URI doesn't match any known concept.
func ResolveConceptURIToName(predicate, appOrigin, uri string) (string, error) {
	concepts, ok := conceptsByPredicate[predicate]
	if !ok {
		return "", fmt.Errorf("predicate %q is not a SKOS concept scheme predicate", predicate)
	}

	appOrigin = strings.TrimSuffix(appOrigin, "/")

	for _, c := range concepts {
		if ConceptURI(appOrigin, predicate, c.Slug) == uri {
			return c.PrefLabel, nil
		}
	}

	return "", fmt.Errorf("no concept found for predicate %q with URI %q", predicate, uri)
}

// SKOSResolveNameToURI is a ResolveNameToURI implementation for SKOS concept scheme predicates.
// It ignores the NameURIResolver (no DB lookup needed) and resolves the name via the in-memory
// concept map, reading APP_ORIGIN from the environment at call time.
func SKOSResolveNameToURI(predicate string) func(NameURIResolver, string) (string, error) {
	return func(_ NameURIResolver, name string) (string, error) {
		appOrigin := os.Getenv("APP_ORIGIN")
		return ResolveSlugToConceptURI(predicate, appOrigin, name)
	}
}

// SKOSResolveURIToName is a ResolveURIToName implementation for SKOS concept scheme predicates.
// It ignores the NameURIResolver and resolves the URI to a prefLabel via the in-memory concept map,
// reading APP_ORIGIN from the environment at call time.
func SKOSResolveURIToName(predicate string) func(NameURIResolver, string) (string, error) {
	return func(_ NameURIResolver, uri string) (string, error) {
		appOrigin := os.Getenv("APP_ORIGIN")
		return ResolveConceptURIToName(predicate, appOrigin, uri)
	}
}
