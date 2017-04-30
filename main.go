package main

import (
	"io"
	"log"
	"net/http"
	"os"
)

func hello(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Hello world!")
}

func main() {
	var port string
	if (len(os.Args) > 1) {
		port = os.Args[1]
	} else {
		port = "8080"
	}
	http.HandleFunc("/", hello)
	log.Printf("Listen on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
