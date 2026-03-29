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
	var result map[string][]TagValueV3
	err = json.Unmarshal(data, &result)
	assertNoError(t, "Unmarshal result failed.", err)
	// Single-value predicates should be arrays of objects
	titleArr := result["title"]
	if len(titleArr) != 1 {
		t.Errorf("Expected 1 title value, got %d", len(titleArr))
	}
	assertEqual(t, "title name", "Blowin' in the Wind", titleArr[0].Name)
}

func TestTagListV3MarshalMultiValue(t *testing.T) {
	tl := TagListV3{
		{PredicateID: "language", Value: "English", URI: "https://eolas.l42.eu/metadata/language/en/"},
		{PredicateID: "language", Value: "French", URI: "https://eolas.l42.eu/metadata/language/fr/"},
		{PredicateID: "title", Value: "Blowin' in the Wind"},
	}
	data, err := json.Marshal(tl)
	assertNoError(t, "MarshalJSON failed.", err)
	var result map[string][]TagValueV3
	err = json.Unmarshal(data, &result)
	assertNoError(t, "Unmarshal result failed.", err)
	// Multi-value predicates should be arrays of objects
	langArr := result["language"]
	if len(langArr) != 2 {
		t.Errorf("Expected 2 language values, got %d", len(langArr))
	}
	assertEqual(t, "language[0].name", "English", langArr[0].Name)
	assertEqual(t, "language[0].uri", "https://eolas.l42.eu/metadata/language/en/", langArr[0].URI)
	assertEqual(t, "language[1].name", "French", langArr[1].Name)
	// Single-value predicate should also be an array
	titleArr := result["title"]
	if len(titleArr) != 1 {
		t.Errorf("Expected 1 title value, got %d", len(titleArr))
	}
	assertEqual(t, "title name", "Blowin' in the Wind", titleArr[0].Name)
}

func TestTagListV3MarshalMultiValueSingleElement(t *testing.T) {
	tl := TagListV3{
		{PredicateID: "composer", Value: "Bob Dylan"},
	}
	data, err := json.Marshal(tl)
	assertNoError(t, "MarshalJSON failed.", err)
	var result map[string][]TagValueV3
	err = json.Unmarshal(data, &result)
	assertNoError(t, "Unmarshal result failed.", err)
	composerArr := result["composer"]
	if len(composerArr) != 1 {
		t.Errorf("Expected 1 composer value, got %d", len(composerArr))
	}
	assertEqual(t, "composer[0].name", "Bob Dylan", composerArr[0].Name)
}

func TestTagListV3MarshalOmitsEmptyURI(t *testing.T) {
	tl := TagListV3{
		{PredicateID: "title", Value: "Test Song"},
	}
	data, err := json.Marshal(tl)
	assertNoError(t, "MarshalJSON failed.", err)
	// Verify the raw JSON doesn't contain "uri" when it's empty
	var raw map[string][]map[string]interface{}
	err = json.Unmarshal(data, &raw)
	assertNoError(t, "Unmarshal raw failed.", err)
	titleObj := raw["title"][0]
	if _, hasURI := titleObj["uri"]; hasURI {
		t.Error("Expected uri to be omitted when empty")
	}
}

func TestTagListV3MarshalEmpty(t *testing.T) {
	tl := TagListV3{}
	data, err := json.Marshal(tl)
	assertNoError(t, "MarshalJSON failed.", err)
	assertEqual(t, "empty TagListV3", "{}", string(data))
}

func TestTagListV3UnmarshalValid(t *testing.T) {
	input := `{"title": [{"name": "Blowin' in the Wind"}], "language": [{"name": "English", "uri": "https://eolas.l42.eu/metadata/language/en/"}, {"name": "French"}], "composer": [{"name": "Bob Dylan"}]}`
	var tl TagListV3
	err := json.Unmarshal([]byte(input), &tl)
	assertNoError(t, "UnmarshalJSON failed.", err)
	// Should have 4 tags: title, language(en), language(fr), composer
	if len(tl) != 4 {
		t.Errorf("Expected 4 tags, got %d", len(tl))
	}
	// Verify URI is preserved
	for _, tag := range tl {
		if tag.PredicateID == "language" && tag.Value == "English" {
			assertEqual(t, "language English URI", "https://eolas.l42.eu/metadata/language/en/", tag.URI)
		}
	}
}

func TestTagListV3UnmarshalRejectMultipleForSingleValue(t *testing.T) {
	// title is single-value, should reject more than one element
	input := `{"title": [{"name": "a"}, {"name": "b"}]}`
	var tl TagListV3
	err := json.Unmarshal([]byte(input), &tl)
	if err == nil {
		t.Error("Expected error when passing multiple values for single-value predicate")
	}
}

func TestTagListV3UnmarshalRejectStringFormat(t *testing.T) {
	// v3 does not accept plain strings — all values must be arrays of objects
	input := `{"title": "a string"}`
	var tl TagListV3
	err := json.Unmarshal([]byte(input), &tl)
	if err == nil {
		t.Error("Expected error when passing string instead of array of objects")
	}
}

func TestTagListV3UnmarshalRejectArrayOfStrings(t *testing.T) {
	// v3 does not accept arrays of strings — values must be objects
	input := `{"language": ["en", "fr"]}`
	var tl TagListV3
	err := json.Unmarshal([]byte(input), &tl)
	if err == nil {
		t.Error("Expected error when passing array of strings instead of array of objects")
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
	original := TagListV3{
		{PredicateID: "title", Value: "Test Song"},
		{PredicateID: "artist", Value: "Test Artist"},
		{PredicateID: "language", Value: "English", URI: "https://eolas.l42.eu/metadata/language/en/"},
		{PredicateID: "language", Value: "German", URI: "https://eolas.l42.eu/metadata/language/de/"},
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
