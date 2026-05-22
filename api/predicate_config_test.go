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
		test.Errorf("Expected 6 multi-value predicates, got %d (album is single-value, so count unchanged)", count)
	}
}

func TestRequiresURIPredicates(test *testing.T) {
	expectedRequiresURI := []string{"language", "about", "mentions", "album"}
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
	if len(predicates) != 4 {
		test.Errorf("Expected 4 RequiresURI predicates, got %d: %v", len(predicates), predicates)
	}
	predicateSet := make(map[string]bool)
	for _, p := range predicates {
		predicateSet[p] = true
	}
	for _, expected := range []string{"language", "about", "mentions", "album"} {
		if !predicateSet[expected] {
			test.Errorf("Expected %q in GetRequiresURIPredicates result, got %v", expected, predicates)
		}
	}
}

func TestAllowedURIHostsSetForRequiresURIPredicates(test *testing.T) {
	for _, pred := range []string{"language", "about", "mentions", "album"} {
		config := GetPredicateConfig(pred)
		if config.AllowedURIHosts == nil {
			test.Errorf("Expected predicate %q to have AllowedURIHosts set", pred)
		}
	}
}

func TestAllowedURIHostsNotSetForNonURIPredicates(test *testing.T) {
	for _, pred := range []string{"composer", "producer", "offence", "title"} {
		config := GetPredicateConfig(pred)
		if config.AllowedURIHosts != nil {
			test.Errorf("Expected predicate %q to have AllowedURIHosts nil", pred)
		}
	}
}

func TestValidateURIHostAcceptsEolasURI(test *testing.T) {
	// eolasOrigin is set to "https://eolas.l42.eu" in TestMain.
	config := GetPredicateConfig("language")
	msg := config.validateURIHost("https://eolas.l42.eu/metadata/language/en/")
	if msg != "" {
		test.Errorf("Expected no error for valid eolas language URI, got: %q", msg)
	}
}

func TestValidateURIHostRejectsWrongHostname(test *testing.T) {
	config := GetPredicateConfig("language")
	msg := config.validateURIHost("https://example.com/metadata/language/en/")
	if msg == "" {
		test.Error("Expected error for language URI with wrong hostname, got empty string")
	}
}

func TestValidateURIHostRejectsNonHttpsScheme(test *testing.T) {
	config := GetPredicateConfig("language")
	msg := config.validateURIHost("http://eolas.l42.eu/metadata/language/en/")
	if msg == "" {
		test.Error("Expected error for language URI with http (not https) scheme, got empty string")
	}
}

func TestValidateURIHostSkipsValidationWhenNoAllowlist(test *testing.T) {
	// Predicates without AllowedURIHosts should pass any URI.
	config := GetPredicateConfig("composer")
	msg := config.validateURIHost("http://anything.example.com/entity/1")
	if msg != "" {
		test.Errorf("Expected no error for predicate without AllowedURIHosts, got: %q", msg)
	}
}

func TestValidateURIHostSkipsValidationWhenAllowlistIsEmpty(test *testing.T) {
	// When mediaMetadataManagerOrigin is empty (as in tests), album's AllowedURIHosts
	// returns [""] which filters to [] — no validation should occur.
	config := GetPredicateConfig("album")
	// Temporarily clear mediaMetadataManagerOrigin to simulate unset env var.
	orig := mediaMetadataManagerOrigin
	mediaMetadataManagerOrigin = ""
	defer func() { mediaMetadataManagerOrigin = orig }()
	msg := config.validateURIHost("/albums/1")
	if msg != "" {
		test.Errorf("Expected no error for album URI when allowlist is empty, got: %q", msg)
	}
}

