package main

import (
	"testing"
)

func TestMultiValuePredicates(test *testing.T) {
	expectedMultiValue := []string{"composer", "producer", "language", "offence", "about", "mentions"}
	for _, pred := range expectedMultiValue {
		if !IsMultiValue(pred) {
			test.Errorf("Expected predicate %q to be multi-value", pred)
		}
	}
}

func TestSingleValuePredicates(test *testing.T) {
	singleValue := []string{"title", "artist", "album", "duration", "img"}
	for _, pred := range singleValue {
		if IsMultiValue(pred) {
			test.Errorf("Expected predicate %q to be single-value", pred)
		}
	}
}

func TestGetPredicateConfigKnown(test *testing.T) {
	config := GetPredicateConfig("composer")
	if !config.MultiValue {
		test.Error("Expected composer config to have MultiValue true")
	}
}

func TestGetPredicateConfigUnknown(test *testing.T) {
	config := GetPredicateConfig("nonexistent")
	if config.MultiValue {
		test.Error("Expected unknown predicate config to have MultiValue false")
	}
}

func TestRegistryCount(test *testing.T) {
	count := 0
	for _, config := range predicateRegistry {
		if config.MultiValue {
			count++
		}
	}
	if count != 6 {
		test.Errorf("Expected 6 multi-value predicates, got %d", count)
	}
}

func TestRequiresURIPredicates(test *testing.T) {
	expectedRequiresURI := []string{"language", "about", "mentions"}
	for _, pred := range expectedRequiresURI {
		config := GetPredicateConfig(pred)
		if !config.RequiresURI {
			test.Errorf("Expected predicate %q to have RequiresURI true", pred)
		}
	}
}

func TestNonURIPredicatesDoNotRequireURI(test *testing.T) {
	nonURIPredicates := []string{"composer", "producer", "offence", "title", "artist"}
	for _, pred := range nonURIPredicates {
		config := GetPredicateConfig(pred)
		if config.RequiresURI {
			test.Errorf("Expected predicate %q to have RequiresURI false", pred)
		}
	}
}

func TestGetRequiresURIPredicates(test *testing.T) {
	predicates := GetRequiresURIPredicates()
	if len(predicates) != 3 {
		test.Errorf("Expected 3 RequiresURI predicates, got %d: %v", len(predicates), predicates)
	}
	predicateSet := make(map[string]bool)
	for _, p := range predicates {
		predicateSet[p] = true
	}
	for _, expected := range []string{"language", "about", "mentions"} {
		if !predicateSet[expected] {
			test.Errorf("Expected %q in GetRequiresURIPredicates result, got %v", expected, predicates)
		}
	}
}
