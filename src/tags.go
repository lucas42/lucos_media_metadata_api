package main

import (
	"errors"
)

type Tag struct {
	TrackID     int
	PredicateID string
	Value       string
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
 * Creates or updates a map of tags
 *
 */
 func (store Datastore) updateTags(trackid int, tags map[string]string) (err error) {
	for predicate, tagValue := range tags {
		if tagValue != "" {
			err = store.updateTag(trackid, predicate, tagValue)
		} else {
			err = store.deleteTag(trackid, predicate)
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
 func (store Datastore) updateTagsIfMissing(trackid int, tags map[string]string) (err error) {
	for predicate, tagValue := range tags {
		if tagValue != "" {
			err = store.updateTagIfMissing(trackid, predicate, tagValue)
		}
		if err != nil {
			return
		}
	}
	return
}

/**
 * Gets all the tags for a given track (in a map of key/value pairs)
 *
 */
func (store Datastore) getAllTagsForTrack(trackid int) (tags map[string]string, err error) {
	tags = make(map[string]string)
	tagList := []Tag{}
	err = store.DB.Select(&tagList, "SELECT * FROM tag WHERE trackid = ?", trackid)
	if err != nil {
		return
	}
	for _, tag := range tagList {
		tags[tag.PredicateID] = tag.Value
	}
	return
}

/**
 * Deletes a given tag
 *
 */
func (store Datastore) deleteTag(trackid int, predicate string) (err error) {
	_, err = store.DB.Exec("DELETE FROM tag WHERE trackid = $1 AND predicateid = $2", trackid, predicate)
	if err != nil && err.Error() == "sql: no rows in result set" {
		err = errors.New("Tag Not Found")
		return
	}
	return
}
