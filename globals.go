package main

import (
	"io"
	"net/http"
	"encoding/json"
	"path"
)

type Global struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

/**
 * Gets data about a global
 *
 */
func (store Datastore) getGlobal(key string) (found bool, global *Global, err error) {
	found = false
	global = new(Global)
	stmt, err := store.DB.Prepare("SELECT key, value FROM global WHERE key = ?")
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
	err = rows.Scan(&global.Key, &global.Value)
	if err != nil {
		return
	}
	return
}

/**
 * Updates a global's value
 *
 */
func (store Datastore) setGlobal(global Global) (err error) {
	stmt, err := store.DB.Prepare("REPLACE INTO global(key, value) values(?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(global.Key, global.Value)
	return err
}


/**
 * Writes a http response with a JSON representation of a global
 */ 
func writeGlobal(store Datastore, w http.ResponseWriter, key string) {
	found, global, err := store.getGlobal(key)
	writeResponse(w, found, "Global Variable", global, err)
}

/**
 * Decodes a JSON representation of a global
 */
func DecodeGlobal(r io.Reader) (Global, error) {
    global := new(Global)
    err := json.NewDecoder(r).Decode(global)
    return *global, err
}

/** 
 * A controller for handling all requests dealing with globals
 */
func (store Datastore) GlobalsController(w http.ResponseWriter, r *http.Request) {
	key := path.Base(r.URL.Path)
	switch r.Method {
		case "PUT":
			global, err := DecodeGlobal(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			global.Key = key
			err = store.setGlobal(global)
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