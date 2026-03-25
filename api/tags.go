package main

import (
	"encoding/json"
	"errors"
	"log/slog"
)

type Tag struct {
	TrackID     int
	PredicateID string
	Value       string
}

// TagList is the internal representation of tags for a track.
// It supports multiple values per predicate, unlike map[string]string.
type TagList []Tag

// MarshalJSON serialises TagList as map[string]string for v2 wire compatibility.
// If multiple tags share a predicate, the last value wins.
func (tl TagList) MarshalJSON() ([]byte, error) {
	m := make(map[string]string, len(tl))
	for _, tag := range tl {
		m[tag.PredicateID] = tag.Value
	}
	return json.Marshal(m)
}

// UnmarshalJSON deserialises a map[string]string into a TagList.
func (tl *TagList) UnmarshalJSON(data []byte) error {
	m := make(map[string]string)
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	*tl = make(TagList, 0, len(m))
	for k, v := range m {
		*tl = append(*tl, Tag{PredicateID: k, Value: v})
	}
	return nil
}

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

/**
 * Get the value for a given tag
 *
 */
func (store Datastore) getTagValue(trackid int, predicate string) (value string, err error) {
	err = store.DB.Get(&value, "SELECT value FROM tag WHERE trackid = $1 AND predicateid = $2", trackid, predicate)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			err = errors.New("Tag Not Found")
		}
	}
	return
}

/**
 * Creates or updates a tag
 *
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
	_, err = store.DB.Exec("REPLACE INTO tag(trackid, predicateid, value) values($1, $2, $3)", trackid, predicate, value)
	return
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
