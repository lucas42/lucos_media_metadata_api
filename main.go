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
			w.Header().Set("Allow", "GET, PUT")
			w.WriteHeader(http.StatusMethodNotAllowed)
			io.WriteString(w, "Error")
	}
}
func routing() *http.ServeMux {
	router := http.NewServeMux()
	router.HandleFunc("/fingerprint/", fingerprint)
	redirect := http.RedirectHandler("/fingerprint/", 307)
	router.Handle("/", redirect)
	return router
}

func main() {
	var port string
	if (len(os.Args) > 1) {
		port = os.Args[1]
	} else {
		port = "8080"
	}
	router := routing()
	log.Printf("Listen on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}
