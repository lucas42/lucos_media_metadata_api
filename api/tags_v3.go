package main

// TagValueV3 is the v3 wire representation of a single tag value.
// All predicates use the same shape: {"name": "...", "uri": "..."}.
// The "uri" field is omitted when empty.
type TagValueV3 struct {
	Name string `json:"name"`
	URI  string `json:"uri,omitempty"`
}
