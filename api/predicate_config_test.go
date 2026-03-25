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
