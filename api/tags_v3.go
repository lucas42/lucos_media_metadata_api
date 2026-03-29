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
type TagListV3 []Tag

// MarshalJSON serialises TagListV3 as a map of predicate to arrays of TagValueV3.
// Every predicate maps to an array, even single-value predicates with one entry.
func (tl TagListV3) MarshalJSON() ([]byte, error) {
	seen := make(map[string][]TagValueV3, len(tl))
	order := make([]string, 0, len(tl))
	for _, tag := range tl {
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
func (tl *TagListV3) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*tl = make(TagListV3, 0, len(raw))
	for pred, val := range raw {
		var arr []TagValueV3
		if err := json.Unmarshal(val, &arr); err != nil {
			return fmt.Errorf("predicate %q must be an array of {\"name\": ..., \"uri\": ...} objects", pred)
		}
		if !IsMultiValue(pred) && len(arr) > 1 {
			return fmt.Errorf("predicate %q is single-value and must have at most one element", pred)
		}
		for _, v := range arr {
			*tl = append(*tl, Tag{PredicateID: pred, Value: v.Name, URI: v.URI})
		}
	}
	return nil
}

// ToTagList converts a TagListV3 to a TagList for use with existing
// datastore methods.
func (tl TagListV3) ToTagList() TagList {
	result := make(TagList, len(tl))
	copy(result, tl)
	return result
}

// TagListToV3 converts a TagList to a TagListV3 for v3 serialisation.
func TagListToV3(tl TagList) TagListV3 {
	result := make(TagListV3, len(tl))
	copy(result, tl)
	return result
}
