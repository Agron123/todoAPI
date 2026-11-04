package main

import (
	"log"
	"net/http"
)

const port = "8080"

func main() {
	serveMux := http.NewServeMux
	server := http.Server{
		Addr:    ":" + port,
		Handler: serveMux(),
	}

	log.Printf("Listening on %v port", port)
	log.Fatal(server.ListenAndServe())
}
