package main

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
