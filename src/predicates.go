package main

import (
	"errors"
	"net/http"
	"path"
)

type Predicate struct {
	ID string `json:"id"`
}

/**
 * Checks if a predicate exists
 *
 */
func (store Datastore) hasPredicate(id string) (found bool, err error) {
	err = store.DB.Get(&found, "SELECT 1 FROM predicate WHERE id = $1", id)
	if err != nil && err.Error() == "sql: no rows in result set" {
		err = nil
	}
	return
}

/**
 * Creates a predicate
 *
 */
func (store Datastore) createPredicate(id string) (err error) {
	_, err = store.DB.Exec("REPLACE INTO predicate(id) values($1)", id)
	return
}

/**
 * Gets data about all predicates in the database
 *
 */
func (store Datastore) getAllPredicates() (predicates []Predicate, err error) {
	predicates = []Predicate{}
	err = store.DB.Select(&predicates, "SELECT id FROM predicate ORDER BY id")
	return
}

/**
 * Writes a http response with a JSON representation of a predicate
 */
func writePredicate(store Datastore, w http.ResponseWriter, id string) {
	found, err := store.hasPredicate(id)
	if err == nil && !found {
		err = errors.New("Predicate Not Found")
	}
	writeJSONResponse(w, Predicate{ID: id}, err)
}

/**
 * Writes a http response with a JSON representation of all predicates
 */
func writeAllPredicates(store Datastore, w http.ResponseWriter) {
	predicates, err := store.getAllPredicates()
	writeJSONResponse(w, predicates, err)
}

/**
 * A controller for handling all requests dealing with predicates directly
 * (Note, manipulation of tags may also impact predicates)
 */
func (store Datastore) PredicatesController(w http.ResponseWriter, r *http.Request) {
	_, id := path.Split(r.URL.Path)
	if id == "" {
		if r.Method == "GET" {
			writeAllPredicates(store, w)
		} else {
			MethodNotAllowed(w, []string{"GET"})
		}
	} else {
		switch r.Method {
		case "PUT":
			err := store.createPredicate(id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			fallthrough
		case "GET":
			writePredicate(store, w, id)
		default:
			MethodNotAllowed(w, []string{"GET", "PUT"})
		}
	}
}
