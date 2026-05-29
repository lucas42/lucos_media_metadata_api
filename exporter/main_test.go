package main

import (
	"testing"
)

// TestPostToScheduleTrackerHandlesBadURL verifies that postToScheduleTracker
// does not panic when it cannot connect — it should log the error and return.
func TestPostToScheduleTrackerHandlesBadURL(t *testing.T) {
	// Should not panic — the function logs the error and returns gracefully.
	postToScheduleTracker("http://127.0.0.1:19999/nowhere", []byte(`{"status":"test"}`))
}
