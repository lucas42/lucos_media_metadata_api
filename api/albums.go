package main

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
)

// AlbumV3 is the v3 wire representation of an album.
type AlbumV3 struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	URI  string `json:"uri"`
}

// AlbumListV3 wraps a paginated list of albums.
type AlbumListV3 struct {
	Albums     []AlbumV3 `json:"albums"`
	TotalPages int       `json:"totalPages"`
	Page       int       `json:"page"`
	TotalItems int       `json:"totalItems"`
}

// albumURI builds the canonical URI for an album using the manager origin.
// The URI uses the manager's path (/albums/{id}) rather than the API's v3 routing
// so that content negotiation can serve an HTML page via the manager.
func (store Datastore) albumURI(id int) string {
	return store.ManagerOrigin + "/albums/" + strconv.Itoa(id)
}

// ParseAlbumIDFromURI extracts the numeric album ID from a URI whose path ends
// with /albums/{id}. Returns an error if the path doesn't match or the id
// is not a positive integer.
func ParseAlbumIDFromURI(uri string) (int, error) {
	// Strip any trailing slash, then take the last path segment.
	trimmed := strings.TrimSuffix(uri, "/")
	idx := strings.LastIndex(trimmed, "/")
	if idx < 0 {
		return 0, errors.New("album URI has no path separator")
	}
	segment := trimmed[idx+1:]
	id, err := strconv.Atoi(segment)
	if err != nil || id <= 0 {
		return 0, errors.New("album URI does not end with a valid album id")
	}
	// Also verify the preceding segment is "albums".
	before := trimmed[:idx]
	if !strings.HasSuffix(before, "/albums") {
		return 0, errors.New("album URI path does not match /albums/{id}")
	}
	return id, nil
}

// getAllAlbums returns a paginated list of albums.
func (store Datastore) getAllAlbums(rawpage string) (list AlbumListV3, err error) {
	const standardLimit = 20

	offset, limit := parsePageParam(rawpage, standardLimit)

	page, parseErr := strconv.Atoi(rawpage)
	if parseErr != nil || page < 1 {
		page = 1
	}

	var total int
	err = store.DB.Get(&total, "SELECT COUNT(*) FROM album")
	if err != nil {
		return
	}

	type albumRow struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}
	var rows []albumRow
	err = store.DB.Select(&rows, "SELECT id, name FROM album ORDER BY name LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return
	}

	albums := make([]AlbumV3, len(rows))
	for i, row := range rows {
		albums[i] = AlbumV3{
			ID:   row.ID,
			Name: row.Name,
			URI:  store.albumURI(row.ID),
		}
	}

	list = AlbumListV3{
		Albums:     albums,
		TotalPages: int(math.Ceil(float64(total) / float64(standardLimit))),
		Page:       page,
		TotalItems: total,
	}
	return
}

// getAlbumByID returns a single album by its integer ID.
func (store Datastore) getAlbumByID(id int) (album AlbumV3, err error) {
	type albumRow struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}
	var row albumRow
	err = store.DB.Get(&row, "SELECT id, name FROM album WHERE id = $1", id)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			err = errors.New("Album Not Found")
		}
		return
	}
	album = AlbumV3{
		ID:   row.ID,
		Name: row.Name,
		URI:  store.albumURI(row.ID),
	}
	return
}

// createAlbum inserts a new album with the given name and returns the created AlbumV3.
// Returns a "duplicate_name" error if the name already exists.
func (store Datastore) createAlbum(name string) (album AlbumV3, err error) {
	slog.Info("Create Album", "name", name)
	result, err := store.DB.Exec("INSERT INTO album(name) VALUES($1)", name)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			err = errors.New("album_duplicate_name")
		}
		return
	}
	id64, err := result.LastInsertId()
	if err != nil {
		return
	}
	album = AlbumV3{
		ID:   int(id64),
		Name: name,
		URI:  store.albumURI(int(id64)),
	}
	return
}

// resolveAlbumNameFromURI extracts the album id from a URI and returns the
// album's name. Used on write to populate the tag value when an album tag
// is written with a URI only.
func (store Datastore) resolveAlbumNameFromURI(uri string) (name string, err error) {
	id, err := ParseAlbumIDFromURI(uri)
	if err != nil {
		return
	}
	album, err := store.getAlbumByID(id)
	if err != nil {
		return
	}
	name = album.Name
	return
}

// AlbumsV3Controller handles all requests to /v3/albums endpoints.
func (store Datastore) AlbumsV3Controller(w http.ResponseWriter, r *http.Request) {
	normalisedpath := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v3/albums"), "/")
	pathparts := strings.Split(normalisedpath, "/")

	slog.Debug("Albums v3 controller", "method", r.Method, "pathparts", pathparts)

	if len(pathparts) <= 1 && pathparts[0] == "" {
		// /v3/albums
		switch r.Method {
		case "GET":
			list, err := store.getAllAlbums(r.URL.Query().Get("page"))
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
			album, err := store.createAlbum(input.Name)
			if err != nil {
				if err.Error() == "album_duplicate_name" {
					writeV3ErrorResponse(w, http.StatusConflict, "An album with that name already exists", "duplicate_name")
					return
				}
				writeV3Error(w, err)
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache, max-age=0, no-store, must-revalidate")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(album)
		default:
			MethodNotAllowed(w, []string{"GET", "POST"})
		}
	} else if len(pathparts) == 2 {
		// /v3/albums/{id}
		id, err := strconv.Atoi(pathparts[1])
		if err != nil || id <= 0 {
			writeV3ErrorResponse(w, http.StatusNotFound, "Album Not Found", "not_found")
			return
		}
		switch r.Method {
		case "GET":
			album, err := store.getAlbumByID(id)
			if err != nil {
				writeV3Error(w, err)
				return
			}
			writeJSONResponse(w, album, nil)
		default:
			MethodNotAllowed(w, []string{"GET"})
		}
	} else {
		writeV3ErrorResponse(w, http.StatusNotFound, "Album Endpoint Not Found", "not_found")
	}
}
