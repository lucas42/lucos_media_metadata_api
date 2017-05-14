package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

func fingerprint(w http.ResponseWriter, r *http.Request) {
	id := strings.Replace(r.URL.Path, "/fingerprint/", "", 1)
	switch r.Method {
		case "GET":
			io.WriteString(w, "Hello fingerprint! "+id)
		case "PUT":
			io.WriteString(w, "PUT not done yet "+id)
		default:
			MethodNotAllowed(w, []string{"GET", "PUT"})
	}
}

func MethodNotAllowed(w http.ResponseWriter, allowedMethods []string) {
	concatMethods := strings.Join(allowedMethods, ", ")
	w.Header().Set("Allow", concatMethods)
	w.WriteHeader(http.StatusMethodNotAllowed)
	io.WriteString(w, "Method Not Allowed, must be one of: "+concatMethods)
}
func routing() *http.ServeMux {
	router := http.NewServeMux()
	router.HandleFunc("/fingerprint/", fingerprint)
	router.HandleFunc("/tracks", trackHandling)
	redirect := http.RedirectHandler("/fingerprint/", 307)
	router.Handle("/", redirect)
	return router
}

func main() {
	var port string
	if (len(os.Getenv("PORT")) > 0) {
		port = os.Getenv("PORT")
	} else {
		port = "8080"
	}
	router := routing()
	log.Printf("Listen on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}
