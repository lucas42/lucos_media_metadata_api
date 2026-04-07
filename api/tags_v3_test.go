package main

import (
	"encoding/json"
	"testing"
)

func TestTagValueV3OmitsEmptyURI(t *testing.T) {
	v := TagValueV3{Name: "Test Song"}
	data, err := json.Marshal(v)
	assertNoError(t, "MarshalJSON failed.", err)
	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	assertNoError(t, "Unmarshal raw failed.", err)
	if _, hasURI := raw["uri"]; hasURI {
		t.Error("Expected uri to be omitted when empty")
	}
	assertEqual(t, "name", "Test Song", raw["name"].(string))
}

func TestTagValueV3IncludesURI(t *testing.T) {
	v := TagValueV3{Name: "English", URI: "https://eolas.l42.eu/metadata/language/en/"}
	data, err := json.Marshal(v)
	assertNoError(t, "MarshalJSON failed.", err)
	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	assertNoError(t, "Unmarshal raw failed.", err)
	assertEqual(t, "name", "English", raw["name"].(string))
	assertEqual(t, "uri", "https://eolas.l42.eu/metadata/language/en/", raw["uri"].(string))
}
