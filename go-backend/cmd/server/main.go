package main

import (
	"go-backend/internal/api"
	"log"
	"net/http"
)

// TODO: Wire config loading, logger, router, and HTTP server startup.

func main() {
	router := api.NewRouter()

	log.Println("Go backend listening on http://localhost:8081")
	if err := http.ListenAndServe(":8081", router); err != nil {
		log.Fatal(err)
	}
}
