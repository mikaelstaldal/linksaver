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
	dataDir := flag.String("data", "data", "directory to store data in")
	basicAuthFile := flag.String("basic-auth-file", "", "Use HTTP basic auth with username and password from given file in htpasswd format (bcrypt only)")
	basicAuthRealm := flag.String("basic-auth-realm", "linksaver", "HTTP basic authentication realm")
	flag.Parse()

	if *port < 1 || *port > 65535 {
		log.Fatalf("Invalid port number: %d. Must be between 1 and 65535", *port)
	}

	info, err := os.Stat(*dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(*dataDir, 0755); err != nil {
				log.Fatalf("Could not create data directory: %s", *dataDir)
			}
		} else {
			log.Fatalf("Failed to access data directory %s: %v", *dataDir, err)
		}
	} else {
		if !info.IsDir() {
			log.Fatalf("Data directory path is not a directory: %s", *dataDir)
		}
	}
	databaseFile := filepath.Join(*dataDir, databaseName)

	info, err = os.Stat(databaseFile)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Fatalf("Failed to access database file %s: %v", databaseFile, err)
		}
	} else {
		if !info.Mode().IsRegular() {
			log.Fatalf("Database file is not a regular file: %s", databaseFile)
		}
	}

	// Initialize database
	database, err := db.InitDB(databaseFile)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	var usernameBcryptHash []byte
	var passwordBcryptHash []byte
	if *basicAuthFile != "" {
		basicAuthContent, err := os.ReadFile(*basicAuthFile)
		if err != nil {
			log.Fatalf("Failed to read basic auth file '%s': %v", *basicAuthFile, err)
		}
		basicAuthStr := strings.TrimSpace(string(basicAuthContent))

		username, passwordBcryptHashStr, ok := strings.Cut(basicAuthStr, ":")
		if !ok {
			log.Fatalf("Invalid basic auth value '%s'", basicAuthStr)
		}
		passwordBcryptHash = []byte(passwordBcryptHashStr)
		_, err = bcrypt.Cost(passwordBcryptHash)
		if err != nil {
			log.Fatalf("Invalid basic auth bcrypt hash '%s': %v", passwordBcryptHashStr, err)
		}

		usernameBcryptHash, err = bcrypt.GenerateFromPassword([]byte(username), bcrypt.MinCost)
		if err != nil {
			log.Fatalf("Failed to hash username: %v", err)
		}

		log.Println("Using HTTP basic authentication")
	}

	// Initialize handlers
	h := web.NewHandlers(executableDir, database, filepath.Join(*dataDir, screenshotsDir), usernameBcryptHash, passwordBcryptHash, *basicAuthRealm)

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
