package main

import (
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
)

/**
 * A struct for holding data about a global
 */
type Global struct {
	Key   string            `json:"key"`
	Value string            `json:"value"`
}

/**
 * Gets data about a global
 *
 */
func (store Datastore) getGlobal(key string) (value string, err error) {
	err = store.DB.Get(&value, "SELECT value FROM global WHERE key = $1", key)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
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
 * Gets data about all global in store
 *
 */
func (store Datastore) getAllGlobals() (globalMap map[string]string, err error) {

	globals := []Global{}
	err = store.DB.Select(&globals, "SELECT key, value FROM global ORDER BY key")
	if err != nil {
		return
	}
	globalMap = make(map[string]string)

	// Loop through all the globals and add them to the map
	for i := range globals {
		global := &globals[i]
		globalMap[global.Key] = global.Value
	}
	return
}

/**
 * Writes a http response with a string representation of a global
 */
func writeGlobal(store Datastore, w http.ResponseWriter, key string) {
	value, err := store.getGlobal(key)
	writePlainResponse(w, value, err)
}


/**
 * Writes a http response with a JSON representation of all globals
 */
func writeAllGlobals(store Datastore, w http.ResponseWriter) {
	globalMap, err := store.getAllGlobals()
	writeJSONResponse(w, globalMap, err)
}

/**
 * A controller for handling all requests dealing with globals
 */
func (store Datastore) GlobalsController(w http.ResponseWriter, r *http.Request) {
	pathparts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathparts) > 1 {
		key := pathparts[1]
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
	} else {
		if r.Method == "GET" {
			writeAllGlobals(store, w)
		} else {
			MethodNotAllowed(w, []string{"GET"})
		}
	}
}
