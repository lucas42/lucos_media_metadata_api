package main

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

// CollectionV3 is the v3 wire representation of a collection.
// Tracks use TrackV3 (structured tags, "id" instead of "trackid", no debug fields).
type CollectionV3 struct {
	Slug        string     `json:"slug"`
	Name        string     `json:"name"`
	Icon        string     `json:"icon"`
	Tracks      *[]TrackV3 `json:"tracks,omitempty"`
	TotalTracks *int       `json:"totalTracks,omitempty"`
	TotalPages  *int       `json:"totalPages,omitempty"`
	IsPlayable  *bool      `json:"isPlayable,omitempty"`
}

// CollectionToV3 converts an internal Collection (with v2 tracks) to a CollectionV3.
func CollectionToV3(c Collection) CollectionV3 {
	v3 := CollectionV3{
		Slug:        c.Slug,
		Name:        c.Name,
		Icon:        c.Icon,
		TotalTracks: c.TotalTracks,
		TotalPages:  c.TotalPages,
		IsPlayable:  c.IsPlayable,
	}
	if c.Tracks != nil {
		v3Tracks := make([]TrackV3, len(*c.Tracks))
		for i, t := range *c.Tracks {
			v3Tracks[i] = TrackToV3(t)
		}
		v3.Tracks = &v3Tracks
	}
	return v3
}

// getCollectionV3 gets a single collection with tracks in V3 format.
func (store Datastore) getCollectionV3(slug string, rawpagenumber string) (collection CollectionV3, err error) {
	v2Collection, err := store.getCollection(slug, rawpagenumber)
	if err != nil {
		return
	}
	collection = CollectionToV3(v2Collection)
	return
}

// getAllCollectionsV3 gets all collections in V3 format (no tracks, just counts).
func (store Datastore) getAllCollectionsV3() (collections []CollectionV3, err error) {
	v2Collections, err := store.getAllCollections()
	if err != nil {
		return
	}
	collections = make([]CollectionV3, len(v2Collections))
	for i, c := range v2Collections {
		collections[i] = CollectionToV3(c)
	}
	return
}

// getRandomTracksInCollectionV3 gets random tracks for a collection in V3 format.
func (store Datastore) getRandomTracksInCollectionV3(slug string, count int) (collection CollectionV3, err error) {
	v2Collection, err := store.getRandomTracksInCollection(slug, count)
	if err != nil {
		return
	}
	collection = CollectionToV3(v2Collection)
	return
}

// CollectionsV3Controller handles all requests to /v3/collections endpoints.
func (store Datastore) CollectionsV3Controller(w http.ResponseWriter, r *http.Request) {
	normalisedpath := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v3/collections"), "/")
	pathparts := strings.Split(normalisedpath, "/")

	slog.Debug("Collections v3 controller", "method", r.Method, "pathparts", pathparts)

	if len(pathparts) <= 1 {
		collections, err := store.getAllCollectionsV3()
		if err != nil {
			writeV3Error(w, err)
			return
		}
		writeJSONResponse(w, collections, nil)
	} else {
		slug := pathparts[1]
		if len(pathparts) == 2 {
			v2Collection, err := store.getBasicCollection(slug)
			if err != nil && r.Method == "PUT" && err.Error() == "Collection Not Found" {
				err = nil
			}
			if err != nil {
				writeV3Error(w, err)
				return
			}
			switch r.Method {
			case "DELETE":
				err = store.deleteCollection(slug)
				writeContentlessResponse(w, err)
			case "PUT":
				var newCollection Collection
				newCollection, err = DecodeCollection(r.Body)
				if err != nil {
					if err.Error() == "EOF" {
						writeV3ErrorResponse(w, http.StatusBadRequest, "No Data Sent", "bad_request")
					} else {
						writeV3Error(w, err)
					}
					return
				}
				newCollection.Slug = slug
				_, _, err = store.updateCreateCollection(v2Collection, newCollection, r.URL.Query().Get("page"))
				if err != nil {
					writeV3Error(w, err)
					return
				}
				fallthrough
			case "GET":
				collection, err := store.getCollectionV3(slug, r.URL.Query().Get("page"))
				if err != nil {
					writeV3Error(w, err)
					return
				}
				writeJSONResponse(w, collection, nil)
			default:
				MethodNotAllowed(w, []string{"GET", "PUT", "DELETE"})
			}
		} else {
			collectionFound, err := store.collectionExists(slug)
			if err != nil {
				writeV3Error(w, err)
				return
			}
			if !collectionFound {
				writeV3Error(w, errors.New("Collection Not Found"))
				return
			}
			trackid, err := strconv.Atoi(pathparts[2])
			if err != nil {
				switch pathparts[2] {
				case "random":
					if r.Method != "GET" {
						MethodNotAllowed(w, []string{"GET"})
						return
					}
					result, err := store.getRandomTracksInCollectionV3(slug, 20)
					if err != nil {
						writeV3Error(w, err)
						return
					}
					writeJSONResponse(w, result, nil)
				default:
					writeV3ErrorResponse(w, http.StatusNotFound, "Collection Endpoint Not Found", "not_found")
				}
			} else {
				trackFound, err := store.trackExists("id", trackid)
				if err != nil {
					writeV3Error(w, err)
					return
				}
				if !trackFound {
					writeV3Error(w, errors.New("Track Not Found"))
					return
				}
				switch r.Method {
				case "GET":
					contains, err := store.isTrackInCollection(slug, trackid)
					if err != nil {
						writeV3Error(w, err)
						return
					}
					if contains {
						writeJSONResponse(w, map[string]bool{"inCollection": true}, nil)
					} else {
						writeV3ErrorResponse(w, http.StatusNotFound, "Track Not In Collection", "not_found")
					}
				case "PUT":
					err = store.addTrackToCollection(slug, trackid)
					if err != nil {
						writeV3Error(w, err)
						return
					}
					writeJSONResponse(w, map[string]bool{"inCollection": true}, nil)
				case "DELETE":
					err = store.removeTrackFromCollection(slug, trackid)
					if err != nil {
						writeV3Error(w, err)
						return
					}
					writeJSONResponse(w, map[string]bool{"inCollection": false}, nil)
				default:
					MethodNotAllowed(w, []string{"GET", "PUT", "DELETE"})
				}
			}
		}
	}
}
