package main

import (
	"testing"

	"lucos_media_metadata_api/predicateconfig"
)

func TestMultiValuePredicates(test *testing.T) {
	expectedMultiValue := []string{"composer", "producer", "language", "offence", "about", "mentions", "theme_tune", "soundtrack"}
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
	if count != 8 {
		test.Errorf("Expected 8 multi-value predicates, got %d (album is single-value, so count unchanged)", count)
	}
}

func TestURIObjectPredicatesHaveValueShapeURIObject(test *testing.T) {
	expectedURIObject := []string{"language", "about", "mentions", "album", "theme_tune", "soundtrack"}
	for _, pred := range expectedURIObject {
		config := GetPredicateConfig(pred)
		if config.ValueShape != predicateconfig.ValueShapeURIObject {
			test.Errorf("Expected predicate %q to have ValueShape URIObject", pred)
		}
	}
}

func TestLiteralPredicatesHaveValueShapeLiteral(test *testing.T) {
	expectedLiteral := []string{"added", "title", "comment", "lyrics", "rating", "memory", "year"}
	for _, pred := range expectedLiteral {
		config := GetPredicateConfig(pred)
		if config.ValueShape != predicateconfig.ValueShapeLiteral {
			test.Errorf("Expected predicate %q to have ValueShape Literal", pred)
		}
	}
}

func TestOmitPredicatesHaveValueShapeOmit(test *testing.T) {
	expectedOmit := []string{"lastSuccessfulPlay", "lastError", "lastSkip", "lastErrorMessage"}
	for _, pred := range expectedOmit {
		config := GetPredicateConfig(pred)
		if config.ValueShape != predicateconfig.ValueShapeOmit {
			test.Errorf("Expected predicate %q to have ValueShape Omit", pred)
		}
	}
}

func TestRequiresURIHelperReturnsTrue(test *testing.T) {
	expectedRequiresURI := []string{"language", "about", "mentions", "album", "theme_tune", "soundtrack"}
	for _, pred := range expectedRequiresURI {
		config := GetPredicateConfig(pred)
		if !config.RequiresURI() {
			test.Errorf("Expected predicate %q RequiresURI() to be true", pred)
		}
	}
}

func TestRequiresURIHelperReturnsFalse(test *testing.T) {
	nonURIPredicates := []string{"composer", "producer", "offence", "title", "artist", "added"}
	for _, pred := range nonURIPredicates {
		config := GetPredicateConfig(pred)
		if config.RequiresURI() {
			test.Errorf("Expected predicate %q RequiresURI() to be false", pred)
		}
	}
}

func TestGetRequiresURIPredicates(test *testing.T) {
	predicates := GetRequiresURIPredicates()
	if len(predicates) != 6 {
		test.Errorf("Expected 6 RequiresURI predicates, got %d: %v", len(predicates), predicates)
	}
	predicateSet := make(map[string]bool)
	for _, p := range predicates {
		predicateSet[p] = true
	}
	for _, expected := range []string{"language", "about", "mentions", "album", "theme_tune", "soundtrack"} {
		if !predicateSet[expected] {
			test.Errorf("Expected %q in GetRequiresURIPredicates result, got %v", expected, predicates)
		}
	}
}

func TestAllowedOriginsSetForURIObjectPredicates(test *testing.T) {
	for _, pred := range []string{"language", "about", "mentions", "album", "theme_tune", "soundtrack"} {
		config := GetPredicateConfig(pred)
		if config.AllowedOrigins == nil {
			test.Errorf("Expected predicate %q to have AllowedOrigins set", pred)
		}
	}
}

func TestAllowedOriginsNotSetForNonURIPredicates(test *testing.T) {
	for _, pred := range []string{"composer", "producer", "offence", "title"} {
		config := GetPredicateConfig(pred)
		if config.AllowedOrigins != nil {
			test.Errorf("Expected predicate %q to have AllowedOrigins nil", pred)
		}
	}
}

func TestValidateURIOriginAcceptsEolasURI(test *testing.T) {
	// eolasOrigin is set to "https://eolas.l42.eu" in TestMain.
	config := GetPredicateConfig("language")
	msg := config.validateURIOrigin("https://eolas.l42.eu/metadata/language/en/")
	if msg != "" {
		test.Errorf("Expected no error for valid eolas language URI, got: %q", msg)
	}
}

func TestValidateURIOriginRejectsWrongOrigin(test *testing.T) {
	config := GetPredicateConfig("language")
	msg := config.validateURIOrigin("https://example.com/metadata/language/en/")
	if msg == "" {
		test.Error("Expected error for language URI with wrong origin, got empty string")
	}
}

func TestValidateURIOriginRejectsNonHttpsScheme(test *testing.T) {
	config := GetPredicateConfig("language")
	// http:// does not match the https:// eolasOrigin prefix.
	msg := config.validateURIOrigin("http://eolas.l42.eu/metadata/language/en/")
	if msg == "" {
		test.Error("Expected error for language URI with http (not https) scheme, got empty string")
	}
}

func TestValidateURIOriginSkipsValidationWhenNoAllowlist(test *testing.T) {
	// Predicates without AllowedOrigins should pass any URI.
	config := GetPredicateConfig("composer")
	msg := config.validateURIOrigin("http://anything.example.com/entity/1")
	if msg != "" {
		test.Errorf("Expected no error for predicate without AllowedOrigins, got: %q", msg)
	}
}

func TestValidateURIOriginSkipsValidationWhenAllowlistIsEmpty(test *testing.T) {
	// mediaMetadataManagerOrigin is "" in tests, so the originMediaMetadataManager
	// constant resolves to "" in validateURIOrigin and is filtered out, leaving no
	// valid origins — validation is skipped entirely.
	config := GetPredicateConfig("album")
	msg := config.validateURIOrigin("/albums/1")
	if msg != "" {
		test.Errorf("Expected no error for album URI when allowlist is empty, got: %q", msg)
	}
}
