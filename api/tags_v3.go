package main

import (
	"encoding/json"
	"fmt"
)

// TagListV3 is the v3 wire representation of tags.
// Multi-value predicates are serialised as JSON arrays.
// Single-value predicates are serialised as JSON strings.
type TagListV3 []Tag

// MarshalJSON serialises TagListV3 with mixed types:
// - single-value predicates → string
// - multi-value predicates → array of strings (even with one value)
func (tl TagListV3) MarshalJSON() ([]byte, error) {
	seen := make(map[string][]string, len(tl))
	order := make([]string, 0, len(tl))
	for _, tag := range tl {
		if _, exists := seen[tag.PredicateID]; !exists {
			order = append(order, tag.PredicateID)
		}
		seen[tag.PredicateID] = append(seen[tag.PredicateID], tag.Value)
	}
	m := make(map[string]interface{}, len(order))
	for _, pred := range order {
		vals := seen[pred]
		if IsMultiValue(pred) {
			m[pred] = vals
		} else {
			m[pred] = vals[len(vals)-1]
		}
	}
	return json.Marshal(m)
}

// UnmarshalJSON deserialises the v3 mixed-type wire format into a TagListV3.
// It validates that single-value predicates are strings and multi-value
// predicates are arrays of strings.
func (tl *TagListV3) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*tl = make(TagListV3, 0, len(raw))
	for pred, val := range raw {
		if IsMultiValue(pred) {
			var arr []string
			if err := json.Unmarshal(val, &arr); err != nil {
				return fmt.Errorf("predicate %q is multi-value and must be a JSON array of strings", pred)
			}
			for _, v := range arr {
				*tl = append(*tl, Tag{PredicateID: pred, Value: v})
			}
		} else {
			var s string
			if err := json.Unmarshal(val, &s); err != nil {
				return fmt.Errorf("predicate %q is single-value and must be a JSON string", pred)
			}
			*tl = append(*tl, Tag{PredicateID: pred, Value: s})
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
