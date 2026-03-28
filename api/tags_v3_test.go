package main

import (
	"encoding/json"
	"testing"
)

func TestTagListV3MarshalSingleValue(t *testing.T) {
	tl := TagListV3{
		{PredicateID: "title", Value: "Blowin' in the Wind"},
		{PredicateID: "artist", Value: "Bob Dylan"},
	}
	data, err := json.Marshal(tl)
	assertNoError(t, "MarshalJSON failed.", err)
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	assertNoError(t, "Unmarshal result failed.", err)
	// Single-value predicates should be strings
	title, ok := result["title"].(string)
	if !ok {
		t.Error("Expected title to be a string")
	}
	assertEqual(t, "title value", "Blowin' in the Wind", title)
}

func TestTagListV3MarshalMultiValue(t *testing.T) {
	tl := TagListV3{
		{PredicateID: "language", Value: "en"},
		{PredicateID: "language", Value: "fr"},
		{PredicateID: "title", Value: "Blowin' in the Wind"},
	}
	data, err := json.Marshal(tl)
	assertNoError(t, "MarshalJSON failed.", err)
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	assertNoError(t, "Unmarshal result failed.", err)
	// Multi-value predicates should be arrays
	langRaw, ok := result["language"].([]interface{})
	if !ok {
		t.Errorf("Expected language to be an array, got %T", result["language"])
	}
	if len(langRaw) != 2 {
		t.Errorf("Expected 2 language values, got %d", len(langRaw))
	}
	assertEqual(t, "language[0]", "en", langRaw[0].(string))
	assertEqual(t, "language[1]", "fr", langRaw[1].(string))
	// Single-value predicate should still be a string
	title, ok := result["title"].(string)
	if !ok {
		t.Error("Expected title to be a string")
	}
	assertEqual(t, "title value", "Blowin' in the Wind", title)
}

func TestTagListV3MarshalMultiValueSingleElement(t *testing.T) {
	// Even with one value, multi-value predicates should be arrays
	tl := TagListV3{
		{PredicateID: "composer", Value: "Bob Dylan"},
	}
	data, err := json.Marshal(tl)
	assertNoError(t, "MarshalJSON failed.", err)
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	assertNoError(t, "Unmarshal result failed.", err)
	composerRaw, ok := result["composer"].([]interface{})
	if !ok {
		t.Errorf("Expected composer to be an array even with one value, got %T", result["composer"])
	}
	if len(composerRaw) != 1 {
		t.Errorf("Expected 1 composer value, got %d", len(composerRaw))
	}
	assertEqual(t, "composer[0]", "Bob Dylan", composerRaw[0].(string))
}

func TestTagListV3MarshalEmpty(t *testing.T) {
	tl := TagListV3{}
	data, err := json.Marshal(tl)
	assertNoError(t, "MarshalJSON failed.", err)
	assertEqual(t, "empty TagListV3", "{}", string(data))
}

func TestTagListV3UnmarshalValid(t *testing.T) {
	input := `{"title": "Blowin' in the Wind", "language": ["en", "fr"], "composer": ["Bob Dylan"]}`
	var tl TagListV3
	err := json.Unmarshal([]byte(input), &tl)
	assertNoError(t, "UnmarshalJSON failed.", err)
	// Should have 4 tags: title, language(en), language(fr), composer
	if len(tl) != 4 {
		t.Errorf("Expected 4 tags, got %d", len(tl))
	}
}

func TestTagListV3UnmarshalRejectArrayForSingleValue(t *testing.T) {
	// title is single-value, should reject array
	input := `{"title": ["a", "b"]}`
	var tl TagListV3
	err := json.Unmarshal([]byte(input), &tl)
	if err == nil {
		t.Error("Expected error when passing array for single-value predicate")
	}
}

func TestTagListV3UnmarshalRejectStringForMultiValue(t *testing.T) {
	// language is multi-value, should reject plain string
	input := `{"language": "en"}`
	var tl TagListV3
	err := json.Unmarshal([]byte(input), &tl)
	if err == nil {
		t.Error("Expected error when passing string for multi-value predicate")
	}
}

func TestTagListV3UnmarshalRejectNumber(t *testing.T) {
	input := `{"title": 42}`
	var tl TagListV3
	err := json.Unmarshal([]byte(input), &tl)
	if err == nil {
		t.Error("Expected error when passing number for single-value predicate")
	}
}

func TestTagListV3UnmarshalRejectArrayOfNumbers(t *testing.T) {
	input := `{"language": [1, 2]}`
	var tl TagListV3
	err := json.Unmarshal([]byte(input), &tl)
	if err == nil {
		t.Error("Expected error when passing array of numbers for multi-value predicate")
	}
}

func TestTagListV3UnmarshalEmptyArray(t *testing.T) {
	input := `{"language": []}`
	var tl TagListV3
	err := json.Unmarshal([]byte(input), &tl)
	assertNoError(t, "UnmarshalJSON failed for empty array.", err)
	if len(tl) != 0 {
		t.Errorf("Expected 0 tags for empty array, got %d", len(tl))
	}
}

func TestTagListV3RoundTrip(t *testing.T) {
	// Create a TagListV3, marshal it, unmarshal it, and check they match
	original := TagListV3{
		{PredicateID: "title", Value: "Test Song"},
		{PredicateID: "artist", Value: "Test Artist"},
		{PredicateID: "language", Value: "en"},
		{PredicateID: "language", Value: "de"},
		{PredicateID: "composer", Value: "Composer A"},
		{PredicateID: "composer", Value: "Composer B"},
	}
	data, err := json.Marshal(original)
	assertNoError(t, "MarshalJSON failed.", err)
	var decoded TagListV3
	err = json.Unmarshal(data, &decoded)
	assertNoError(t, "UnmarshalJSON failed.", err)
	if len(decoded) != len(original) {
		t.Errorf("Expected %d tags after round-trip, got %d", len(original), len(decoded))
	}
}
