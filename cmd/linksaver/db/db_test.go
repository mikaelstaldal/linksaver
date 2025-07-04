package db

import (
	"os"
	"testing"
)

//goland:noinspection GoDirectComparisonOfErrors
func TestDB(t *testing.T) {
	// Use a temporary database file for testing
	dbFile := "test.database"

	// Initialize the database
	database, err := InitDB(dbFile)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	t.Cleanup(func() {
		_ = database.Close()
		_ = os.Remove(dbFile)
	})

	// Test adding a link
	url := "https://example.com"
	title := "Example Website"
	id, err := database.AddLink(url, title)
	if err != nil {
		t.Fatalf("Failed to add link: %v", err)
	}
	if id <= 0 {
		t.Fatalf("Expected positive ID, got %d", id)
	}

	// Test adding duplicate link
	_, err = database.AddLink(url, "bogus")
	if err != ErrDuplicate {
		t.Fatalf("Expected error adding duplicate link")
	}

	// Test getting all links
	links, err := database.GetAllLinks()
	if err != nil {
		t.Fatalf("Failed to get links: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("Expected 1 link, got %d", len(links))
	}
	if links[0].URL != url {
		t.Errorf("Expected URL %s, got %s", url, links[0].URL)
	}
	if links[0].Title != title {
		t.Errorf("Expected title %s, got %s", title, links[0].Title)
	}
	if links[0].AddedAt.IsZero() {
		t.Errorf("Expected non-zero AddedAt")
	}

	// Test successful retrieval
	link, err := database.GetLink(id)
	if err != nil {
		t.Errorf("Failed to get link: %v", err)
	}
	if link.URL != url {
		t.Errorf("Expected single URL %s, got %s", url, link.URL)
	}
	if link.Title != title {
		t.Errorf("Expected single title %s, got %s", title, link.Title)
	}
	if link.AddedAt.IsZero() {
		t.Errorf("Expected single non-zero AddedAt")
	}

	// Test non-existent link
	_, err = database.GetLink(99999)
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound for fetching non-existent link, got: %v", err)
	}

	// Test deleting a link
	err = database.DeleteLink(id)
	if err != nil {
		t.Fatalf("Failed to delete link: %v", err)
	}

	// Test deleting a non-existing link
	err = database.DeleteLink(9999)
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound for deleting non-existent link, got: %v", err)
	}

	// Verify the link was deleted
	links, err = database.GetAllLinks()
	if err != nil {
		t.Fatalf("Failed to get links after deletion: %v", err)
	}
	if len(links) != 0 {
		t.Fatalf("Expected 0 links after deletion, got %d", len(links))
	}
}
