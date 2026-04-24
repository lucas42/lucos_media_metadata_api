package main

import (
	"errors"
	"log/slog"
	"strings"
)

type Tag struct {
	TrackID     int
	PredicateID string
	Value       string
	URI         string `db:"uri"`
}

// TagList is the internal representation of tags for a track.
// It supports multiple values per predicate.
type TagList []Tag

// GetValue returns the value for the first tag matching the given predicate,
// or an empty string if not found.
func (tl TagList) GetValue(predicate string) string {
	for _, tag := range tl {
		if tag.PredicateID == predicate {
			return tag.Value
		}
	}
	return ""
}

// GetValues returns all values for the given predicate.
func (tl TagList) GetValues(predicate string) []string {
	var vals []string
	for _, tag := range tl {
		if tag.PredicateID == predicate {
			vals = append(vals, tag.Value)
		}
	}
	return vals
}

// SetValue sets the value for a predicate. If the predicate already exists,
// it updates the first occurrence. Otherwise it appends a new tag.
func (tl *TagList) SetValue(predicate string, value string) {
	for i, tag := range *tl {
		if tag.PredicateID == predicate {
			(*tl)[i].Value = value
			return
		}
	}
	*tl = append(*tl, Tag{PredicateID: predicate, Value: value})
}

// TagValueV3 is the v3 wire representation of a single tag value.
// All predicates use the same shape: {"name": "...", "uri": "..."}.
// The "uri" field is omitted when empty.
type TagValueV3 struct {
	Name string `json:"name"`
	URI  string `json:"uri,omitempty"`
}

/**
 * Get the value for a given tag
 *
 */
func (store Datastore) getTagValue(trackid int, predicate string) (value string, err error) {
	var values []string
	err = store.DB.Select(&values, "SELECT value FROM tag WHERE trackid = $1 AND predicateid = $2", trackid, predicate)
	if err != nil {
		return
	}
	if len(values) == 0 {
		err = errors.New("Tag Not Found")
		return
	}
	value = strings.Join(values, ",")
	return
}

/**
 * Creates or updates a tag
 *
 * For multi-value predicates (as defined in predicateRegistry), comma-separated
 * values are split and stored as separate rows.
 */
func (store Datastore) updateTag(trackid int, predicate string, value string) (err error) {
	slog.Info("Update Tag", "trackid", trackid, "predicate", predicate, "value", value)
	trackFound, err := store.trackExists("id", trackid)
	if err != nil {
		return
	}
	if !trackFound {
		err = errors.New("Unknown Track")
		return
	}
	hasPredicate, err := store.hasPredicate(predicate)
	if err != nil {
		return
	}
	if !hasPredicate {
		err = store.createPredicate(predicate)
		if err != nil {
			return
		}
	}

	// Split comma-separated values for multi-value predicates.
	values := []string{value}
	if IsMultiValue(predicate) && strings.Contains(value, ",") {
		values = splitCSV(value)
	}

	// Delete existing value(s) for this predicate then insert the new one(s).
	// Wrapped in a transaction so the tag is not lost if the INSERT fails.
	tx, err := store.DB.Beginx()
	if err != nil {
		return
	}
	_, err = tx.Exec("DELETE FROM tag WHERE trackid = $1 AND predicateid = $2", trackid, predicate)
	if err != nil {
		_ = tx.Rollback()
		return
	}
	for _, v := range values {
		_, err = tx.Exec("INSERT INTO tag(trackid, predicateid, value) VALUES($1, $2, $3)", trackid, predicate, v)
		if err != nil {
			_ = tx.Rollback()
			return
		}
	}
	err = tx.Commit()
	return
}

// splitCSV splits a comma-separated string into trimmed, non-empty parts.
func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

/**
 * Creates a tag if the track doesn't already have one with the given predicate
 *
 */
func (store Datastore) updateTagIfMissing(trackid int, predicate string, value string) (err error) {
	_, err = store.getTagValue(trackid, predicate)
	if err != nil && err.Error() == "Tag Not Found" {
		err = store.updateTag(trackid, predicate, string(value))
	}
	return
}

/**
 * Creates or updates a set of tags
 *
 */
func (store Datastore) updateTags(trackid int, tags TagList) (err error) {
	for _, tag := range tags {
		if tag.Value != "" {
			err = store.updateTag(trackid, tag.PredicateID, tag.Value)
		} else {
			err = store.deleteTag(trackid, tag.PredicateID)
		}
		if err != nil {
			return
		}
	}
	return
}

/**
 * Creates only the given tags which the given track doesn't already have
 *
 */
func (store Datastore) updateTagsIfMissing(trackid int, tags TagList) (err error) {
	for _, tag := range tags {
		if tag.Value != "" {
			err = store.updateTagIfMissing(trackid, tag.PredicateID, tag.Value)
		}
		if err != nil {
			return
		}
	}
	return
}

/**
 * Gets all the tags for a given track
 *
 */
func (store Datastore) getAllTagsForTrack(trackid int) (tags TagList, err error) {
	tags = TagList{}
	err = store.DB.Select(&tags, "SELECT * FROM tag WHERE trackid = ?", trackid)
	return
}

/**
 * Deletes a given tag
 *
 */
func (store Datastore) deleteTag(trackid int, predicate string) (err error) {
	slog.Info("Delete Tag", "trackid", trackid, "predicate", predicate)
	_, err = store.DB.Exec("DELETE FROM tag WHERE trackid = $1 AND predicateid = $2", trackid, predicate)
	if err != nil && err.Error() == "sql: no rows in result set" {
		err = errors.New("Tag Not Found")
		return
	}
	return
}
