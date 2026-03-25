package main

// PredicateConfig holds metadata about a predicate's behaviour.
// Currently governs multi-value storage; designed to be extended
// with RDF mapping, form rendering hints, etc.
type PredicateConfig struct {
	// MultiValue indicates this predicate can have multiple values
	// per track. When true, the database allows multiple tag rows
	// for the same (trackid, predicateid) pair, and the v3 API
	// serialises/deserialises the values as a JSON array.
	MultiValue bool
}

// predicateRegistry defines per-predicate configuration.
// Predicates not listed here use default behaviour (single-value).
var predicateRegistry = map[string]PredicateConfig{
	"composer": {MultiValue: true},
	"producer": {MultiValue: true},
	"language": {MultiValue: true},
	"offence":  {MultiValue: true},
	"about":    {MultiValue: true},
	"mentions": {MultiValue: true},
}

// GetPredicateConfig returns the configuration for a predicate.
// Predicates not in the registry return a zero-value config
// (single-value, default behaviour).
func GetPredicateConfig(predicateID string) PredicateConfig {
	if config, ok := predicateRegistry[predicateID]; ok {
		return config
	}
	return PredicateConfig{}
}

// IsMultiValue is a convenience for the most common check.
func IsMultiValue(predicateID string) bool {
	return GetPredicateConfig(predicateID).MultiValue
}
