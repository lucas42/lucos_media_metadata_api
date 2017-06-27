package main

import (
	"io/ioutil"
	"net/http"
	"path"
)

/**
 * Gets data about a global
 *
 */
func (store Datastore) getGlobal(key string) (found bool, value string, err error) {
	found = false
	value = ""
	stmt, err := store.DB.Prepare("SELECT value FROM global WHERE key = ?")
	if err != nil {
		return
	}
	defer stmt.Close()
	rows, err := stmt.Query(key)
	if err != nil {
		return
	}
	defer rows.Close()

	result := rows.Next()
	if (result == false) {
		return
	}
	found = true
	err = rows.Scan(&value)
	if err != nil {
		return
	}
	return
}

/**
 * Updates a global's value
 *
 */
func (store Datastore) setGlobal(key string, value string) (err error) {
	stmt, err := store.DB.Prepare("REPLACE INTO global(key, value) values(?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(key, value)
	return err
}


/**
 * Writes a http response with a JSON representation of a global
 */ 
func writeGlobal(store Datastore, w http.ResponseWriter, key string) {
	found, value, err := store.getGlobal(key)
	writePlainResponse(w, found, "Global Variable", value, err)
}

/** 
 * A controller for handling all requests dealing with globals
 */
func (store Datastore) GlobalsController(w http.ResponseWriter, r *http.Request) {
	key := path.Base(r.URL.Path)
	switch r.Method {
		case "PUT":
			value, err := ioutil.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			err = store.setGlobal(key, string(value))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			fallthrough
		case "GET":
			writeGlobal(store, w, key)
		default:
			MethodNotAllowed(w, []string{"GET", "PUT"})
	}
}