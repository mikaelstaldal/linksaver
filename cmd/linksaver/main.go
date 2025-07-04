package main

import (
	"flag"
	"fmt"
	"github.com/mikaelstaldal/linksaver/cmd/linksaver/db"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	// Define command line flags
	port := flag.Int("port", 8080, "port to listen on")
	flag.Parse()

	if *port < 1 || *port > 65535 {
		log.Fatalf("Invalid port number: %d. Must be between 1 and 65535", *port)
	}

	// Initialize database
	database, err := db.InitDB("linksaver.sqlite")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	if err := os.MkdirAll("screenshots", 0755); err != nil {
		log.Fatalf("failed to create screenshots directory: %v", err)
	}

	// Initialize handlers
	h := NewHandler(database)

	// Start server
	serverAddr := fmt.Sprintf(":%d", *port)
	log.Printf("Starting server on %s", serverAddr)
	server := http.Server{
		Addr:         serverAddr,
		Handler:      h.Routes(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 20 * time.Second,
		IdleTimeout:  time.Minute,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
