package main

import (
	"errors"
	"io/ioutil"
	"net/http"
	"path"
)

/**
 * Gets data about a global
 *
 */
func (store Datastore) getGlobal(key string) (value string, err error) {
	err = store.DB.Get(&value, "SELECT value FROM global WHERE key = $1", key)
	if err != nil {
		if (err.Error() == "sql: no rows in result set") {
			err = errors.New("Global Variable Not Found")
		}
	}
	return
}

/**
 * Updates a global's value
 *
 */
func (store Datastore) setGlobal(key string, value string) (err error) {
	_, err = store.DB.Exec("REPLACE INTO global(key, value) values($1, $2)", key, value)
	return
}


/**
 * Writes a http response with a JSON representation of a global
 */ 
func writeGlobal(store Datastore, w http.ResponseWriter, key string) {
	value, err := store.getGlobal(key)
	writePlainResponse(w, true, "Global Variable", value, err)
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