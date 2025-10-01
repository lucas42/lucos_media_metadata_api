package main

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
	"lucos_media_metadata_api/rdfgen"
)

/**
 * A struct for holding data about a given track
 */
type Track struct {
	Fingerprint string            `json:"fingerprint"`
	Duration    int               `json:"duration"`
	URL         string            `json:"url"`
	ID          int               `json:"trackid"`
	Tags        map[string]string `json:"tags"`
	Weighting   float64           `json:"weighting"`
	RandWeight  float64           `json:"_random_weighting,omitempty"` // Only for debug purposes
	Cumweight   float64           `json:"_cum_weighting,omitempty"`    // Only for debug purposes
	Collections *[]Collection     `json:"collections,omitempty"`
}

func (track Track) getName() (string) {
	if track.Tags["title"] != "" {
		return "\""+track.Tags["title"]+"\""
	}
	return "#"+strconv.Itoa(track.ID)
}
func (original Track) updateNeeded (changeSet Track, onlyMissing bool) bool {
	if changeSet.Fingerprint != "" && changeSet.Fingerprint != original.Fingerprint{
		return true
	}
	if changeSet.Duration != 0 && changeSet.Duration != original.Duration{
		return true
	}
	if changeSet.URL != "" && changeSet.URL != original.URL{
		return true
	}
	if changeSet.Weighting != 0 && changeSet.Weighting != original.Weighting{
		return true
	}
	for predicate, changeValue := range changeSet.Tags {

		// If only missing tags are being updated, then ignore any
		// predicates where the original track already has a value
		if onlyMissing && original.Tags[predicate] != "" {
			continue
		}
		if original.Tags[predicate] != changeValue {
			return true
		}
	}
	if changeSet.Collections != nil {
		if (original.Collections == nil) {
			return true
		}
		// Check for collections which are on the existing track but not the new one
		for _, existingCollection := range *original.Collections {
			remove := true
			for _, newCollection := range *changeSet.Collections {
				if existingCollection.Slug == newCollection.Slug {
					remove = false
				}
			}
			if remove {
				return true
			}
		}
		// Check for collections which are on the new track and not the existing one
		for _, newCollection := range *changeSet.Collections {
			add := true
			for _, existingCollection := range *original.Collections {
				if existingCollection.Slug == newCollection.Slug {
					add = false
				}
			}
			if add {
				return true
			}
		}
	}
	return false
}

/**
 * Updates or Creates fields about a track based on a given field
 *
 */
func (store Datastore) updateCreateTrackDataByField(filterField string, value interface{}, track Track, existingTrack Track, onlyMissing bool) (storedTrack Track, action string, err error) {
	// If no changes are needed, return the existing track
	if !existingTrack.updateNeeded(track, onlyMissing) {
		slog.Debug("Update track not needed", "filterField", filterField, "value", value, "onlyMissing", onlyMissing)
		return existingTrack, "noChange", nil
	}

	slog.Info("update/create track", "filterField", filterField, "value", value)
	action = "Changed"
	updateFields := []string{}
	if track.Duration != 0 {
		updateFields = append(updateFields, "duration = :duration")
	}
	if track.URL != "" {
		err = store.checkForDuplicateTrack("url", track.URL, filterField, value)
		if err != nil {
			return
		}
		updateFields = append(updateFields, "url = :url")
	}
	if track.Fingerprint != "" {
		err = store.checkForDuplicateTrack("fingerprint", track.Fingerprint, filterField, value)
		if err != nil {
			return
		}
		updateFields = append(updateFields, "fingerprint = :fingerprint")
	}
	if existingTrack.ID > 0 {
		if len(updateFields) > 0 {
			_, err = store.DB.NamedExec("UPDATE TRACK SET "+strings.Join(updateFields, ", ")+" WHERE "+filterField+" = :"+filterField, track)
		}
	} else {
		_, err = store.DB.NamedExec("INSERT INTO track(duration, url, fingerprint) values(:duration, :url, :fingerprint)", track)
	}
	if err != nil {
		return
	}
	storedTrack, err = store.getTrackDataByField(filterField, value)
	if err != nil {
		return
	}
	if track.Tags == nil {
		track.Tags = map[string]string{}
	}
	if existingTrack.Tags["added"] == "" && track.Tags["added"] == "" {
		track.Tags["added"] = time.Now().Format(time.RFC3339)
	}
	if !onlyMissing {
		err = store.updateTags(storedTrack.ID, track.Tags);
	} else {
		err = store.updateTagsIfMissing(storedTrack.ID, track.Tags);
	}
	if err != nil {
		return
	}

	// If Collections are given, add and remove collection to match
	// Only looks at the collection's slug to identify it.  All other fields are ignored.
	if track.Collections != nil {
		if (existingTrack.Collections != nil) {
			// Check for collections which are on the existing track but not the new one, and remove them
			for _, existingCollection := range *existingTrack.Collections {
				remove := true
				for _, newCollection := range *track.Collections {
					if existingCollection.Slug == newCollection.Slug {
						remove = false
					}
				}
				if remove {
					err = store.removeTrackFromCollection(existingCollection.Slug, storedTrack.ID)
					if err != nil {
						return
					}
				}
			}
			// Check for collections which are on the new track and not the existing one, and add them
			for _, newCollection := range *track.Collections {
				add := true
				for _, existingCollection := range *existingTrack.Collections {
					if existingCollection.Slug == newCollection.Slug {
						add = false
					}
				}
				if add {
					err = store.addTrackToCollection(newCollection.Slug, storedTrack.ID)
					if err != nil {
						return
					}
				}
			}
		} else {
			// If there's no existing Collections add all the new ones
			for _, newCollection := range *track.Collections {
				err = store.addTrackToCollection(newCollection.Slug, storedTrack.ID)
				if err != nil {
					return
				}
			}

		}
	}

	storedTrack, err = store.getTrackDataByField(filterField, value)
	if existingTrack.ID > 0 {
		action = "trackUpdated"
		store.Loganne.post(action, "Track "+storedTrack.getName()+" updated", storedTrack, existingTrack)
	} else {
		action = "trackAdded"
		store.Loganne.post(action, "New Track "+storedTrack.getName()+" added", storedTrack, existingTrack)

	}
	return
}

/**
 * Gets data about a track for a given value of a given field
 *
 */
func (store Datastore) getTrackDataByField(field string, value interface{}) (track Track, err error) {
	track = Track{}
	err = store.DB.Get(&track, "SELECT id, url, fingerprint, duration, weighting FROM track WHERE "+field+"=$1", value)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			err = errors.New("Track Not Found")
		}
		return
	}
	track.Tags, err = store.getAllTagsForTrack(track.ID)
	if err != nil {
		return
	}
	collections, err := store.getCollectionsByTrack(track.ID)
	track.Collections = &collections
	return
}

/**
 * Checks whether a track exists based on a given field
 *
 */
func (store Datastore) trackExists(field string, value interface{}) (found bool, err error) {
	err = store.DB.Get(&found, "SELECT 1 FROM track WHERE "+field+"=$1", value)
	if err != nil && err.Error() == "sql: no rows in result set" {
		err = nil
	}
	return
}
/**
 * Checks whether any other tracks are duplicating a given field
 *
 */
func (store Datastore) checkForDuplicateTrack(compareField string, compareValue interface{}, filterField string, filterValue interface{}) (err error) {
	var trackid int
	err = store.DB.Get(&trackid, "SELECT id FROM track WHERE "+compareField+" = $1 AND "+filterField+" != $2", compareValue, filterValue)
	if err != nil && err.Error() == "sql: no rows in result set" {
		err = nil
	}
	if (trackid != 0) {
		err = errors.New("Duplicate: track "+strconv.Itoa(trackid)+" has same "+compareField)
	}
	return
}


/**
 * Sets the weighting for a given track
 *
 */
func (store Datastore) setTrackWeighting(trackid int, newWeighting float64) (err error) {
	existingTrack, err := store.getTrackDataByField("id", trackid)
	if err != nil {
		return
	}
	oldWeighting := existingTrack.Weighting

	// No need to do anything if old and new values are the same
	if newWeighting == oldWeighting {
		return
	}
	slog.Info("Set Track Weighting", "trackid", trackid, "oldWeighting", oldWeighting, "newWeighting", newWeighting)

	// Any tracks currently with a higher cumulative weighting than this one should be shmooshed down to remove this one
	_, err = store.DB.Exec("UPDATE track SET cum_weighting = cum_weighting - $1 WHERE cum_weighting >= (SELECT cum_weighting FROM track WHERE id = $2)", oldWeighting, trackid)
	if err != nil {
		return
	}
	var newCumulativeWeighting float64

	// If there's a non zero weighting, then stick this track to the end of the cumulative weighting list
	if newWeighting > 0 {
		var max float64
		max, err = store.getMaxCumWeighting()
		if err != nil {
			return
		}
		newCumulativeWeighting = max + newWeighting

	// If the weighting is zero, then set the cumulative weighting to zero too, to avoid 2 tracks with the same weighting
	} else {
		newCumulativeWeighting = 0
	}
	_, err = store.DB.Exec("UPDATE track SET weighting = $1, cum_weighting = $2 WHERE id = $3", newWeighting, newCumulativeWeighting, trackid)
	if err != nil {
		return
	}
	err = store.updateTrackAllCollectionsCumWeighting(trackid, oldWeighting, newWeighting)
	if err != nil {
		return
	}
	updatedTrack := existingTrack
	updatedTrack.Weighting = newWeighting
	humanReadableMessage := "Weighting for track "+updatedTrack.getName()+" updated from "+strconv.FormatFloat(oldWeighting, 'f', -1, 64)+" to "+strconv.FormatFloat(newWeighting, 'f', -1, 64)
	store.Loganne.post("trackWeightingUpdated", humanReadableMessage, updatedTrack, existingTrack)
	return
}

/**
 * Gets the weighting of a given track
 *
 */
func (store Datastore) getTrackWeighting(trackid int) (weighting float64, err error) {
	err = store.DB.Get(&weighting, "SELECT weighting FROM track WHERE id=$1", trackid)
	if err != nil && err.Error() == "sql: no rows in result set" {
		err = errors.New("Track Not Found")
	}
	return
}

/**
 * Gets the highest cumulative weighting value (defaults to 0)
 *
 */
func (store Datastore) getMaxCumWeighting() (maxcumweighting float64, err error) {
	err = store.DB.Get(&maxcumweighting, "SELECT IFNULL(MAX(cum_weighting), 0) FROM track")
	if maxcumweighting < 0 {
		err = errors.New("cum_weightings are negative, max: " + strconv.FormatFloat(maxcumweighting, 'f', -1, 64))
	}
	return
}


/**
 * Gets data about a random set of tracks from the database
 *
 */
func (store Datastore) getRandomTracks(count int) (tracks []Track, err error) {
	max, err := store.getMaxCumWeighting()
	if err != nil {
		return
	}

	tracks = []Track{}
	if max == 0 {
		return
	}

	for i := 0; i < count; i++ {
		track := Track{}
		weighting := rand.Float64() * max
		err = store.DB.Get(&track, "SELECT id, url, fingerprint, duration, weighting, cum_weighting AS cumweight FROM track WHERE cum_weighting > $1 ORDER BY cum_weighting ASC LIMIT 1", weighting)
		track.RandWeight = weighting
		if err != nil {
			return
		}
		track.Tags, err = store.getAllTagsForTrack(track.ID)
		if err != nil {
			return
		}
		tracks = append(tracks, track)
	}
	return
}

/**
 * Deletes a given track and all its tags
 *
 */
func (store Datastore) deleteTrack(trackid int) (err error) {
	slog.Info("Delete Track", "trackid", trackid)

	// Set the track's weighting to zero before deletion
	// So that the cumulative weighting of higher tracks get updated appropriately
	err = store.setTrackWeighting(trackid, 0)
	if (err != nil) {
		return
	}
	existingTrack, err := store.getTrackDataByField("id", trackid)
	if (err != nil) {
		return
	}
	_, err = store.DB.Exec("DELETE FROM tag WHERE trackid=$1", trackid)
	if (err != nil) {
		return
	}

	// Loop through each collection and remove separately, so that cum_weightings get updated appropriately
	collections, err := store.getCollectionsByTrack(trackid)
	if (err != nil) {
		return
	}
	for _, collection := range collections {
		err = store.removeTrackFromCollection(collection.Slug, trackid)
		if (err != nil) {
			return
		}
	}

	_, err = store.DB.Exec("DELETE FROM track WHERE id=$1", trackid)
	if (err != nil) {
		return
	}
	store.Loganne.post("trackDeleted", "Track "+existingTrack.getName()+" deleted", Track{}, existingTrack)
	return
}

/**
 * Writes a http response with a JSON representation of a given track
 */
func writeTrackDataByField(store Datastore, w http.ResponseWriter, field string, value interface{}) {
	track, err := store.getTrackDataByField(field, value)
	writeJSONResponse(w, track, err)
}

/**
 * Writes a http response with a RDF representation of a given track
 */
func writeTrackRDFByField(store Datastore, w http.ResponseWriter, field string, value interface{}, rdfType string) {
	rows, err := store.DB.Query(`
		SELECT t.id, t.url, t.duration, tg.predicateid, tg.value
		FROM track t
		LEFT JOIN tag tg ON tg.trackid = t.id WHERE `+field+"=$1", value)
	defer rows.Close()
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			err = errors.New("Track Not Found")
		}
		return
	}
	graph, err := rdfgen.TrackToRdf(rows)
	if err != nil {
		return
	}
	writeRDFResponse(w, graph, rdfType, err)
}

/**
 * Writes a http response with the weighting of a given track
 */
func writeWeighting(store Datastore, w http.ResponseWriter, trackid int) {
	weighting, err := store.getTrackWeighting(trackid)
	weightingval := strconv.FormatFloat(weighting, 'f', -1, 64)
	writePlainResponse(w, weightingval, err)
}

/**
 * Write a http response with a JSON representation of a random 20 tracks
 */
func writeRandomTracks(store Datastore, w http.ResponseWriter) {
	tracks, err := store.getRandomTracks(20)
	writeJSONResponse(w, tracks, err)
}


/**
 * Deletes a given track and writes a response with no content
 */
func deleteTrackHandler(store Datastore, w http.ResponseWriter, trackid int) {
	err := store.deleteTrack(trackid)
	w.Header().Set("Track-Action", "trackDeleted")
	writeContentlessResponse(w, err)
}

/**
 * Decodes a JSON representation of a track
 */
func DecodeTrack(r io.Reader) (Track, error) {
	track := new(Track)
	err := json.NewDecoder(r).Decode(track)
	return *track, err
}
