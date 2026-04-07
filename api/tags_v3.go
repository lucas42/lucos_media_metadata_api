package main

import (
	"encoding/json"
	"fmt"
)

// TagValueV3 is the v3 wire representation of a single tag value.
// All predicates use the same shape: {"name": "...", "uri": "..."}.
// The "uri" field is omitted when empty.
type TagValueV3 struct {
	Name string `json:"name"`
	URI  string `json:"uri,omitempty"`
}

// TagListV3 is the v3 wire representation of tags.
// All predicates are serialised as arrays of TagValueV3 objects,
// whether single-value or multi-value.
//
// ExplicitPredicates tracks which predicates were explicitly included in the
// incoming JSON (including those with empty arrays, which signal "clear this
// field"). It is nil when the struct was not populated from JSON (e.g. when
// "tags" was absent from a PATCH body).
type TagListV3 struct {
	Tags               []Tag
	ExplicitPredicates map[string]struct{}
}

// IsPresent reports whether this TagListV3 was explicitly included in a JSON
// request body.  It returns false for the zero value (Tags absent from body).
func (tl TagListV3) IsPresent() bool {
	return tl.ExplicitPredicates != nil
}

// MarshalJSON serialises TagListV3 as a map of predicate to arrays of TagValueV3.
// Every predicate maps to an array, even single-value predicates with one entry.
// ExplicitPredicates is an internal field and is never included in output.
func (tl TagListV3) MarshalJSON() ([]byte, error) {
	seen := make(map[string][]TagValueV3, len(tl.Tags))
	order := make([]string, 0, len(tl.Tags))
	for _, tag := range tl.Tags {
		if _, exists := seen[tag.PredicateID]; !exists {
			order = append(order, tag.PredicateID)
		}
		seen[tag.PredicateID] = append(seen[tag.PredicateID], TagValueV3{
			Name: tag.Value,
			URI:  tag.URI,
		})
	}
	m := make(map[string][]TagValueV3, len(order))
	for _, pred := range order {
		m[pred] = seen[pred]
	}
	return json.Marshal(m)
}

// UnmarshalJSON deserialises the v3 structured format into a TagListV3.
// All predicates must be arrays of objects with at least a "name" field.
// Predicates with empty arrays are recorded in ExplicitPredicates so callers
// can distinguish "field not in request" from "field explicitly cleared".
func (tl *TagListV3) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	tl.Tags = make([]Tag, 0, len(raw))
	tl.ExplicitPredicates = make(map[string]struct{}, len(raw))
	for pred, val := range raw {
		tl.ExplicitPredicates[pred] = struct{}{}
		var arr []TagValueV3
		if err := json.Unmarshal(val, &arr); err != nil {
			return fmt.Errorf("predicate %q must be an array of {\"name\": ..., \"uri\": ...} objects", pred)
		}
		if !IsMultiValue(pred) && len(arr) > 1 {
			return fmt.Errorf("predicate %q is single-value and must have at most one element", pred)
		}
		for _, v := range arr {
			tl.Tags = append(tl.Tags, Tag{PredicateID: pred, Value: v.Name, URI: v.URI})
		}
	}
	return nil
}

// ToTagList converts a TagListV3 to a TagList for use with existing
// datastore methods.
func (tl TagListV3) ToTagList() TagList {
	result := make(TagList, len(tl.Tags))
	copy(result, tl.Tags)
	return result
}

// TagListToV3 converts a TagList to a TagListV3 for v3 serialisation.
// ExplicitPredicates is left nil since this conversion is for outbound
// responses, not inbound requests.
func TagListToV3(tl TagList) TagListV3 {
	result := TagListV3{Tags: make([]Tag, len(tl))}
	copy(result.Tags, tl)
	return result
}
