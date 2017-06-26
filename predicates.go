package main

import (
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
	found = false
	stmt, err := store.DB.Prepare("SELECT 1 FROM predicate WHERE id = ?")
	if err != nil {
		return
	}
	defer stmt.Close()
	rows, err := stmt.Query(id)
	if err != nil {
		return
	}
	defer rows.Close()

	result := rows.Next()
	found = (result != false)
	return
}

/**
 * Creates a predicate
 *
 */
func (store Datastore) createPredicate(id string) (err error) {
	stmt, err := store.DB.Prepare("REPLACE INTO predicate(id) values(?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(id)
	return err
}

/**
 * Gets data about all predicates in the database
 *
 */
func (store Datastore) getAllPredicates() (predicates []*Predicate, err error) {
	predicates = []*Predicate{}
	rows, err := store.DB.Query("SELECT id FROM predicate")
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		predicate := new(Predicate)
		err = rows.Scan(&predicate.ID)
		if err != nil {
			return
		}
		predicates = append(predicates, predicate)
	}
	err = rows.Err()
	if err != nil {
		return
	}
	return
}

/**
 * Writes a http response with a JSON representation of a predicate
 */ 
func writePredicate(store Datastore, w http.ResponseWriter, id string) {
	found, err := store.hasPredicate(id)
	writeResponse(w, found, "Predicate", Predicate{ID: id}, err)
}

/**
 * Writes a http response with a JSON representation of all predicates
 */ 
func writeAllPredicates(store Datastore, w http.ResponseWriter) {
	predicates, err := store.getAllPredicates()
	writeResponse(w, true, "Predicate", predicates, err)
}

/** 
 * A controller for handling all requests dealing with predicates directly
 * (Note, manipulation of tags may also impact predicates)
 */
func (store Datastore) PredicatesController(w http.ResponseWriter, r *http.Request) {
	_, id := path.Split(r.URL.Path)
	if (id == "") {
		if (r.Method == "GET") {
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