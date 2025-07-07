package main

import (
	"flag"
	"fmt"
	"github.com/mikaelstaldal/linksaver/cmd/linksaver/db"
	"github.com/mikaelstaldal/linksaver/cmd/linksaver/web"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const databaseName = "linksaver.sqlite"
const screenshotsDir = "screenshots"

func main() {
	// Determine the path of executable
	executablePath, err := os.Executable()
	if err != nil {
		log.Fatalf("could not determine executable path: %v", err)
	}
	executableDir := filepath.Dir(executablePath)

	// Define command line flags
	port := flag.Int("port", 8080, "port to listen on")
	flag.Parse()

	if *port < 1 || *port > 65535 {
		log.Fatalf("Invalid port number: %d. Must be between 1 and 65535", *port)
	}

	// Initialize database
	database, err := db.InitDB(databaseName)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	if err := os.MkdirAll(screenshotsDir, 0755); err != nil {
		log.Fatalf("failed to create screenshots directory: %v", err)
	}

	// Initialize handlers
	h := web.NewHandlers(executableDir, database, screenshotsDir)

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
