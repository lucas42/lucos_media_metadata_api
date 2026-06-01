package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"

	"lucos_media_metadata_api/rdfgen"
)

// ArtistV3 is the v3 wire representation of an artist.
type ArtistV3 struct {
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	URI       string  `json:"uri"`
	PersonURI *string `json:"personUri,omitempty"` // optional eolas:Person identity link (ADR-0009)
}

// validateArtistPersonURI checks that personURI, if non-empty, starts with the
// configured eolas origin (host-validated per ADR-0005/#245). Returns a non-empty
// error message suitable for a 400 response if validation fails, empty string if OK.
func validateArtistPersonURI(personURI string) string {
	if eolasOrigin == "" {
		return "EOLAS_ORIGIN not configured; cannot validate personUri"
	}
	if !strings.HasPrefix(personURI, eolasOrigin+"/") {
		return fmt.Sprintf("personUri must start with the eolas origin (%s/)", eolasOrigin)
	}
	return ""
}

// ArtistListV3 wraps a paginated list of artists.
type ArtistListV3 struct {
	Artists    []ArtistV3 `json:"artists"`
	TotalPages int        `json:"totalPages"`
	Page       int        `json:"page"`
	TotalItems int        `json:"totalItems"`
}

// artistURI builds the canonical URI for an artist using the manager origin.
func (store Datastore) artistURI(id int) string {
	return store.ManagerOrigin + "/artists/" + strconv.Itoa(id)
}

// ParseArtistIDFromURI extracts the numeric artist ID from a URI whose path ends
// with /artists/{id}. Returns an error if the path doesn't match or the id
// is not a positive integer.
func ParseArtistIDFromURI(uri string) (int, error) {
	trimmed := strings.TrimSuffix(uri, "/")
	idx := strings.LastIndex(trimmed, "/")
	if idx < 0 {
		return 0, errors.New("artist URI has no path separator")
	}
	segment := trimmed[idx+1:]
	id, err := strconv.Atoi(segment)
	if err != nil || id <= 0 {
		return 0, errors.New("artist URI does not end with a valid artist id")
	}
	before := trimmed[:idx]
	if !strings.HasSuffix(before, "/artists") {
		return 0, errors.New("artist URI path does not match /artists/{id}")
	}
	return id, nil
}

// getAllArtists returns a paginated list of artists, optionally filtered by a
// case-insensitive substring match on name via the q parameter.
func (store Datastore) getAllArtists(rawpage, q string) (list ArtistListV3, err error) {
	const standardLimit = 20

	offset, limit := parsePageParam(rawpage, standardLimit)

	page, parseErr := strconv.Atoi(rawpage)
	if parseErr != nil || page < 1 {
		page = 1
	}

	type artistRow struct {
		ID        int            `db:"id"`
		Name      string         `db:"name"`
		PersonURI sql.NullString `db:"person_uri"`
	}

	var total int
	var rows []artistRow
	if q != "" {
		like := "%" + q + "%"
		err = store.DB.Get(&total, "SELECT COUNT(*) FROM artist WHERE name LIKE $1", like)
		if err != nil {
			return
		}
		err = store.DB.Select(&rows, "SELECT id, name, person_uri FROM artist WHERE name LIKE $1 ORDER BY name LIMIT $2 OFFSET $3", like, limit, offset)
	} else {
		err = store.DB.Get(&total, "SELECT COUNT(*) FROM artist")
		if err != nil {
			return
		}
		err = store.DB.Select(&rows, "SELECT id, name, person_uri FROM artist ORDER BY name LIMIT $1 OFFSET $2", limit, offset)
	}
	if err != nil {
		return
	}

	artists := make([]ArtistV3, len(rows))
	for i, row := range rows {
		artists[i] = ArtistV3{
			ID:        row.ID,
			Name:      row.Name,
			URI:       store.artistURI(row.ID),
			PersonURI: nullStringToPtr(row.PersonURI),
		}
	}

	list = ArtistListV3{
		Artists:    artists,
		TotalPages: int(math.Ceil(float64(total) / float64(standardLimit))),
		Page:       page,
		TotalItems: total,
	}
	return
}

// getArtistByID returns a single artist by its integer ID.
func (store Datastore) getArtistByID(id int) (artist ArtistV3, err error) {
	type artistRow struct {
		ID        int            `db:"id"`
		Name      string         `db:"name"`
		PersonURI sql.NullString `db:"person_uri"`
	}
	var row artistRow
	err = store.DB.Get(&row, "SELECT id, name, person_uri FROM artist WHERE id = $1", id)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			err = errors.New("Artist Not Found")
		}
		return
	}
	artist = ArtistV3{
		ID:        row.ID,
		Name:      row.Name,
		URI:       store.artistURI(row.ID),
		PersonURI: nullStringToPtr(row.PersonURI),
	}
	return
}

// nullStringToPtr converts a sql.NullString to a *string.
// Returns nil for NULL or empty strings (both mean "no value set").
func nullStringToPtr(ns sql.NullString) *string {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	s := ns.String
	return &s
}

// createArtist inserts a new artist with the given name and returns the created ArtistV3.
// Returns an "artist_duplicate_name" error if the name already exists.
func (store Datastore) createArtist(name string) (artist ArtistV3, err error) {
	slog.Info("Create Artist", "name", name)
	result, err := store.DB.Exec("INSERT INTO artist(name) VALUES($1)", name)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			err = errors.New("artist_duplicate_name")
		}
		return
	}
	id64, err := result.LastInsertId()
	if err != nil {
		return
	}
	artist = ArtistV3{
		ID:   int(id64),
		Name: name,
		URI:  store.artistURI(int(id64)),
	}
	store.Loganne.artistPost("artistCreated", "Artist \""+name+"\" created", artist, true)
	return
}

// updateArtist renames an existing artist and optionally updates the eolas:Person
// identity link (ADR-0009).
//
// personURI semantics:
//   - nil: do not change the existing person_uri value
//   - pointer to "": clear person_uri (set to NULL — un-curate the identity link)
//   - pointer to non-empty string: set person_uri to that value
//
// Returns "Artist Not Found" if the id doesn't exist.
func (store Datastore) updateArtist(id int, name string, personURI *string) (artist ArtistV3, err error) {
	slog.Info("Update Artist", "id", id, "name", name)

	tx, err := store.DB.Beginx()
	if err != nil {
		return
	}
	defer func() { _ = tx.Rollback() }()

	var result sql.Result
	if personURI == nil {
		// Don't touch person_uri
		result, err = tx.Exec("UPDATE artist SET name = $1 WHERE id = $2", name, id)
	} else {
		// Set or clear person_uri
		var personURIVal sql.NullString
		if *personURI != "" {
			personURIVal = sql.NullString{String: *personURI, Valid: true}
		}
		result, err = tx.Exec("UPDATE artist SET name = $1, person_uri = $2 WHERE id = $3", name, personURIVal, id)
	}
	if err != nil {
		_ = tx.Rollback()
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			err = errors.New("artist_duplicate_name")
		}
		return
	}
	rows, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return
	}
	if rows == 0 {
		_ = tx.Rollback()
		err = errors.New("Artist Not Found")
		return
	}

	// Cascade the name change to all tag rows referencing this artist.
	artistURI := store.artistURI(id)
	_, err = tx.Exec(
		"UPDATE tag SET value = $1 WHERE predicateid = 'artist' AND uri = $2",
		name, artistURI,
	)
	if err != nil {
		_ = tx.Rollback()
		return
	}

	err = tx.Commit()
	if err != nil {
		return
	}

	// Re-fetch the full artist row so PersonURI reflects the DB state.
	artist, err = store.getArtistByID(id)
	if err != nil {
		return
	}
	store.Loganne.artistPost("artistUpdated", "Artist \""+name+"\" updated", artist, true)
	return
}

// deleteArtist removes an artist by id.
// Returns an error if any tracks reference it.
func (store Datastore) deleteArtist(id int) error {
	slog.Info("Delete Artist", "id", id)
	artist, err := store.getArtistByID(id)
	if err != nil {
		return err
	}
	var count int
	err = store.DB.Get(&count, "SELECT COUNT(*) FROM tag WHERE predicateid = 'artist' AND uri = $1", artist.URI)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("artist_in_use")
	}
	_, err = store.DB.Exec("DELETE FROM artist WHERE id = $1", id)
	if err != nil {
		return err
	}
	store.Loganne.artistPost("artistDeleted", "Artist \""+artist.Name+"\" deleted", artist, false)
	return nil
}

// mergeArtists merges one or more source artists into the target artist.
func (store Datastore) mergeArtists(targetID int, sourceIDs []int) (artist ArtistV3, err error) {
	slog.Info("Merge Artists", "targetID", targetID, "sourceIDs", sourceIDs)

	for _, id := range sourceIDs {
		if id == targetID {
			err = errors.New("merge_target_in_sources")
			return
		}
	}

	target, err := store.getArtistByID(targetID)
	if err != nil {
		return
	}

	sources := make([]ArtistV3, len(sourceIDs))
	for i, id := range sourceIDs {
		var src ArtistV3
		src, err = store.getArtistByID(id)
		if err != nil {
			return
		}
		sources[i] = src
	}

	targetURI := store.artistURI(targetID)

	tx, err := store.DB.Beginx()
	if err != nil {
		return
	}
	defer func() { _ = tx.Rollback() }()

	for _, src := range sources {
		_, err = tx.Exec(
			"UPDATE tag SET uri = $1, value = $2 WHERE predicateid = 'artist' AND uri = $3",
			targetURI, target.Name, src.URI,
		)
		if err != nil {
			_ = tx.Rollback()
			return
		}
		_, err = tx.Exec("DELETE FROM artist WHERE id = $1", src.ID)
		if err != nil {
			_ = tx.Rollback()
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		return
	}

	for _, src := range sources {
		store.Loganne.artistMergedPost("artistMerged", "Artist \""+src.Name+"\" merged into \""+target.Name+"\"", src, target)
	}

	artist = target
	return
}

// ResolveOrCreateArtistByName satisfies the predicateconfig.NameURIResolver interface.
// It looks up an artist by name (creating it if absent) and returns its URI.
func (store Datastore) ResolveOrCreateArtistByName(name string) (string, error) {
	artist, err := store.resolveOrCreateArtistByName(name)
	if err != nil {
		return "", err
	}
	return artist.URI, nil
}

// ResolveArtistNameFromURI satisfies the predicateconfig.NameURIResolver interface.
// It extracts the artist id from a URI and returns the artist's name.
func (store Datastore) ResolveArtistNameFromURI(uri string) (string, error) {
	return store.resolveArtistNameFromURI(uri)
}

// resolveOrCreateArtistByName looks up an artist by name. If no artist with that
// name exists, one is created.
func (store Datastore) resolveOrCreateArtistByName(name string) (artist ArtistV3, err error) {
	type artistRow struct {
		ID        int            `db:"id"`
		Name      string         `db:"name"`
		PersonURI sql.NullString `db:"person_uri"`
	}
	var row artistRow
	err = store.DB.Get(&row, "SELECT id, name, person_uri FROM artist WHERE name = $1", name)
	if err == nil {
		artist = ArtistV3{ID: row.ID, Name: row.Name, URI: store.artistURI(row.ID), PersonURI: nullStringToPtr(row.PersonURI)}
		return
	}
	if err.Error() != "sql: no rows in result set" {
		return
	}
	err = nil
	return store.createArtist(name)
}

// resolveArtistNameFromURI extracts the artist id from a URI and returns the
// artist's name.
func (store Datastore) resolveArtistNameFromURI(uri string) (name string, err error) {
	id, err := ParseArtistIDFromURI(uri)
	if err != nil {
		return
	}
	artist, err := store.getArtistByID(id)
	if err != nil {
		return
	}
	name = artist.Name
	return
}

// writeArtistRDFByID writes an RDF representation of a single artist.
func writeArtistRDFByID(store Datastore, w http.ResponseWriter, id int, rdfType string) {
	_, err := store.getArtistByID(id)
	if err != nil {
		writeRDFResponse(w, nil, rdfType, err)
		return
	}
	rows, err := store.DB.Query("SELECT id, name, person_uri FROM artist WHERE id = $1", id)
	if err != nil {
		writeRDFResponse(w, nil, rdfType, err)
		return
	}
	defer rows.Close()
	graph, err := rdfgen.ArtistToRdf(rows)
	writeRDFResponse(w, graph, rdfType, err)
}

// ArtistsV3Controller handles all requests to /v3/artists endpoints.
func (store Datastore) ArtistsV3Controller(w http.ResponseWriter, r *http.Request) {
	normalisedpath := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v3/artists"), "/")
	pathparts := strings.Split(normalisedpath, "/")

	slog.Debug("Artists v3 controller", "method", r.Method, "pathparts", pathparts)

	if len(pathparts) <= 1 && pathparts[0] == "" {
		// /v3/artists
		switch r.Method {
		case "GET":
			list, err := store.getAllArtists(r.URL.Query().Get("page"), r.URL.Query().Get("q"))
			if err != nil {
				writeV3Error(w, err)
				return
			}
			writeJSONResponse(w, list, nil)
		case "POST":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				writeV3Error(w, err)
				return
			}
			var input struct {
				Name string `json:"name"`
			}
			if err = json.Unmarshal(body, &input); err != nil || input.Name == "" {
				writeV3ErrorResponse(w, http.StatusBadRequest, "Request body must include a non-empty \"name\" field", "bad_request")
				return
			}
			artist, err := store.createArtist(input.Name)
			if err != nil {
				if err.Error() == "artist_duplicate_name" {
					writeV3ErrorResponse(w, http.StatusConflict, "An artist with that name already exists", "duplicate_name")
					return
				}
				writeV3Error(w, err)
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache, max-age=0, no-store, must-revalidate")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(artist)
		default:
			MethodNotAllowed(w, []string{"GET", "POST"})
		}
	} else if len(pathparts) == 2 && pathparts[1] == "merge" {
		// /v3/artists/merge
		if r.Method != "POST" {
			MethodNotAllowed(w, []string{"POST"})
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeV3Error(w, err)
			return
		}
		var input struct {
			TargetID  int   `json:"targetId"`
			SourceIDs []int `json:"sourceIds"`
		}
		if err = json.Unmarshal(body, &input); err != nil || input.TargetID <= 0 || len(input.SourceIDs) == 0 {
			writeV3ErrorResponse(w, http.StatusBadRequest, "Request body must include a positive \"targetId\" and a non-empty \"sourceIds\" array", "bad_request")
			return
		}
		artist, err := store.mergeArtists(input.TargetID, input.SourceIDs)
		if err != nil {
			if err.Error() == "merge_target_in_sources" {
				writeV3ErrorResponse(w, http.StatusBadRequest, "Target artist cannot also be listed as a source", "bad_request")
				return
			}
			writeV3Error(w, err)
			return
		}
		writeJSONResponse(w, artist, nil)
	} else if len(pathparts) == 2 {
		// /v3/artists/{id}
		id, err := strconv.Atoi(pathparts[1])
		if err != nil || id <= 0 {
			writeV3ErrorResponse(w, http.StatusNotFound, "Artist Not Found", "not_found")
			return
		}
		switch r.Method {
		case "GET":
			isRDF, mime := prefersRDF(r)
			if isRDF {
				writeArtistRDFByID(store, w, id, mime)
				return
			}
			artist, err := store.getArtistByID(id)
			if err != nil {
				writeV3Error(w, err)
				return
			}
			writeJSONResponse(w, artist, nil)
		case "PUT":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				writeV3Error(w, err)
				return
			}
			var input struct {
				Name      string  `json:"name"`
				PersonURI *string `json:"personUri"`
			}
			if err = json.Unmarshal(body, &input); err != nil || input.Name == "" {
				writeV3ErrorResponse(w, http.StatusBadRequest, "Request body must include a non-empty \"name\" field", "bad_request")
				return
			}
			// Validate personUri if provided and non-empty
			if input.PersonURI != nil && *input.PersonURI != "" {
				if msg := validateArtistPersonURI(*input.PersonURI); msg != "" {
					writeV3ErrorResponse(w, http.StatusBadRequest, msg, "invalid_person_uri")
					return
				}
			}
			artist, err := store.updateArtist(id, input.Name, input.PersonURI)
			if err != nil {
				if err.Error() == "artist_duplicate_name" {
					writeV3ErrorResponse(w, http.StatusConflict, "An artist with that name already exists", "duplicate_name")
					return
				}
				writeV3Error(w, err)
				return
			}
			writeJSONResponse(w, artist, nil)
		case "DELETE":
			err := store.deleteArtist(id)
			if err != nil {
				if err.Error() == "artist_in_use" {
					writeV3ErrorResponse(w, http.StatusConflict, "Artist is referenced by one or more tracks", "in_use")
					return
				}
				writeV3Error(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			MethodNotAllowed(w, []string{"GET", "PUT", "DELETE"})
		}
	} else {
		writeV3ErrorResponse(w, http.StatusNotFound, "Artist Endpoint Not Found", "not_found")
	}
}
