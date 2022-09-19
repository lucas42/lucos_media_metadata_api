package main

import (
    "testing"
)

/**
 * Checks that the homepage redirects to /tracks
 */
func TestHomepage(t *testing.T) {
	checkRedirect(t, "/", "/tracks")
}

/**
 * Checks that unknown urls return a 404
 */
func TestNotFound(t *testing.T) {
	makeRequest(t, "GET", "/404", "", 404, "404 page not found\n", false)
}

