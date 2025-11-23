package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mikaelstaldal/linksaver/cmd/linksaver/db"
	"github.com/mikaelstaldal/linksaver/cmd/linksaver/web"
	"golang.org/x/crypto/bcrypt"
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
	addr := flag.String("addr", "", "address to listen on")
	flag.Parse()

	if *port < 1 || *port > 65535 {
		log.Fatalf("Invalid port number: %d. Must be between 1 and 65535", *port)
	}

	var usernameBcryptHash []byte
	var passwordBcryptHash []byte
	basicAuth := os.Getenv("BASIC_AUTH")
	if basicAuth != "" {
		username, passwordBcryptHashStr, ok := strings.Cut(basicAuth, ":")
		if !ok {
			log.Fatalf("Invalid BASIC_AUTH value '%s'", basicAuth)
		}
		passwordBcryptHash = []byte(passwordBcryptHashStr)
		_, err := bcrypt.Cost(passwordBcryptHash)
		if err != nil {
			log.Fatalf("Invalid BASIC_AUTH bcrypt hash '%s': %v", passwordBcryptHashStr, err)
		}

		usernameBcryptHash, err = bcrypt.GenerateFromPassword([]byte(username), bcrypt.MinCost)
		if err != nil {
			log.Fatalf("Failed to hash username: %v", err)
		}

		log.Println("Using HTTP basic authentication")
	}

	// Initialize database
	database, err := db.InitDB(databaseName)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Initialize handlers
	h := web.NewHandlers(executableDir, database, screenshotsDir, usernameBcryptHash, passwordBcryptHash)

	// Start server
	serverAddr := fmt.Sprintf("%s:%d", *addr, *port)
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
