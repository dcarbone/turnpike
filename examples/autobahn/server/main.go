package main

import (
	"log"
	"net/http"

	"github.com/dcarbone/turnpike"
)

func main() {
	turnpike.Debug()
	s := turnpike.NewBasicWebSocketServer("turnpike.examples")
	server := &http.Server{
		Handler: s,
		Addr:    ":8000",
	}
	log.Println("turnpike server starting on port 8000")
	log.Fatal(server.ListenAndServe())
}
