package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

func hello(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Hello world!")
}
func fingerprint(w http.ResponseWriter, r *http.Request) {
	id := strings.Replace(r.URL.Path, "/fingerprint/", "", 1)
	switch r.Method {
		case "GET":
			io.WriteString(w, "Hello fingerprint! "+id)
		case "PUT":
			io.WriteString(w, "PUT not done yet "+id)
		default:
			w.Header().Set("Allow", "GET, PUT")
			w.WriteHeader(http.StatusMethodNotAllowed)
			io.WriteString(w, "Error ")
	}
}

func main() {
	var port string
	if (len(os.Args) > 1) {
		port = os.Args[1]
	} else {
		port = "8080"
	}
	http.HandleFunc("/fingerprint/", fingerprint)
	http.HandleFunc("/", hello)
	log.Printf("Listen on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
