package main
import (
	"strconv"
	"testing"
)

func assertWeighting(test *testing.T, store Datastore, trackid int, expectedWeighting float64, expectedCumulativeWeighting float64) {
	actualWeighting, err := store.getTrackWeighting(trackid)
	if err != nil {
		test.Errorf("Error getting weighting for track %d: %s", trackid, err.Error())
	}
	actualCumulativeWeighting := 0.0
	err = store.DB.Get(&actualCumulativeWeighting, "SELECT cum_weighting FROM track WHERE id=$1", trackid)
	if err != nil {
		test.Errorf("Error getting cumulative weighting for track %d: %s", trackid, err.Error())
	}
	assertEqual(test, "Incorrect Weighting for track "+strconv.Itoa(trackid), expectedWeighting, actualWeighting)
	assertEqual(test, "Incorrect Cumulative Weighting for track "+strconv.Itoa(trackid), expectedCumulativeWeighting, actualCumulativeWeighting)
}

func TestSetWeighting(test *testing.T) {
	clearData()
	store := DBInit("testweighting.sqlite", MockLoganne{})
	store.DB.Exec("INSERT INTO track")
	_, err := store.DB.Exec("INSERT INTO track (url,fingerprint,duration) values ('/track1','abc',3)")
	if err != nil {
		test.Errorf("Error inserting track 1: %s", err.Error())
	}
	err = store.setTrackWeighting(1, 5)
	if err != nil {
		test.Errorf("Error setting weighting 1: %s", err.Error())
	}
	assertWeighting(test, store, 1, 5, 5)
}
func TestDeletingTrackCorrectsWeighting(test *testing.T) {
	clearData()
	store := DBInit("testweighting.sqlite", MockLoganne{})
	_, err := store.DB.Exec("INSERT INTO track (url,fingerprint,duration) values ('/track1','abc',3),('/track2','def',6),('/track3','hij',9)")
	if err != nil {
		test.Errorf("Error inserting track 1: %s", err.Error())
	}
	err = store.setTrackWeighting(1, 5)
	if err != nil {
		test.Errorf("Error setting weighting 1: %s", err.Error())
	}
	err = store.setTrackWeighting(2, 5)
	if err != nil {
		test.Errorf("Error setting weighting 2: %s", err.Error())
	}
	err = store.setTrackWeighting(3, 5)
	if err != nil {
		test.Errorf("Error setting weighting 3: %s", err.Error())
	}
	assertWeighting(test, store, 1, 5, 5)
	assertWeighting(test, store, 2, 5, 10)
	assertWeighting(test, store, 3, 5, 15)
	err = store.deleteTrack(2)
	if err != nil {
		test.Errorf("Error deleting track 2: %s", err.Error())
	}
	assertWeighting(test, store, 1, 5, 5)
	assertWeighting(test, store, 3, 5, 10)
}

func TestChangeWeighting(test *testing.T) {
	clearData()
	store := DBInit("testweighting.sqlite", MockLoganne{})
	_, err := store.DB.Exec("INSERT INTO track (url,fingerprint,duration) values ('/track1','abc',3),('/track2','def',6),('/track3','hij',9)")
	if err != nil {
		test.Errorf("Error inserting track 1: %s", err.Error())
	}
	err = store.setTrackWeighting(1, 5)
	if err != nil {
		test.Errorf("Error setting weighting 1: %s", err.Error())
	}
	err = store.setTrackWeighting(2, 5)
	if err != nil {
		test.Errorf("Error setting weighting 2: %s", err.Error())
	}
	err = store.setTrackWeighting(3, 5)
	if err != nil {
		test.Errorf("Error setting weighting 3: %s", err.Error())
	}
	assertWeighting(test, store, 1, 5, 5)
	assertWeighting(test, store, 2, 5, 10)
	assertWeighting(test, store, 3, 5, 15)

	err = store.setTrackWeighting(2, 7)
	if err != nil {
		test.Errorf("Error setting track 2 to zero weighting: %s", err.Error())
	}
	assertWeighting(test, store, 1, 5, 5)
	assertWeighting(test, store, 3, 5, 10)
	assertWeighting(test, store, 2, 7, 17)
}
func TestZeroWeightingAfterDelete(test *testing.T) {
	clearData()
	store := DBInit("testweighting.sqlite", MockLoganne{})
	_, err := store.DB.Exec("INSERT INTO track (url,fingerprint,duration) values ('/track1','abc',3),('/track2','def',6),('/track3','hij',9),('/track4','klm',12)")
	if err != nil {
		test.Errorf("Error inserting track 1: %s", err.Error())
	}
	err = store.setTrackWeighting(1, 5)
	if err != nil {
		test.Errorf("Error setting weighting 1: %s", err.Error())
	}
	err = store.setTrackWeighting(2, 5)
	if err != nil {
		test.Errorf("Error setting weighting 2: %s", err.Error())
	}
	err = store.setTrackWeighting(3, 5)
	if err != nil {
		test.Errorf("Error setting weighting 3: %s", err.Error())
	}
	err = store.setTrackWeighting(4, 5)
	if err != nil {
		test.Errorf("Error setting weighting 4: %s", err.Error())
	}
	assertWeighting(test, store, 1, 5, 5)
	assertWeighting(test, store, 2, 5, 10)
	assertWeighting(test, store, 3, 5, 15)
	assertWeighting(test, store, 4, 5, 20)

	err = store.deleteTrack(2)
	if err != nil {
		test.Errorf("Error deleting track 2: %s", err.Error())
	}
	err = store.setTrackWeighting(3, 0)
	if err != nil {
		test.Errorf("Error setting track 3 to zero weighting: %s", err.Error())
	}
	assertWeighting(test, store, 1, 5, 5)
	assertWeighting(test, store, 4, 5, 10)
	assertWeighting(test, store, 3, 0, 0)
}