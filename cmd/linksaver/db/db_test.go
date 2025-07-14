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
	description := "This is an example website"
	body := "<body><p>Some peculiar text in the body</p></body>"
	id, err := database.AddLink(url, title, description, []byte(body))
	if err != nil {
		t.Fatalf("Failed to add link: %v", err)
	}
	if id <= 0 {
		t.Fatalf("Expected positive ID, got %d", id)
	}

	// Test adding another link
	url2 := "https://other.com"
	title2 := "Fun page"
	description2 := "Here some completely different content"
	body2 := "<body><p>Other body data</p></body>"
	id2, err := database.AddLink(url2, title2, description2, []byte(body2))
	if err != nil {
		t.Fatalf("Failed to add link 2: %v", err)
	}
	if id2 <= 0 {
		t.Fatalf("Expected positive ID, got %d", id)
	}
	if id2 == id {
		t.Fatalf("Expected different id")
	}

	// Test adding duplicate link
	_, err = database.AddLink(url, "bogus", "", nil)
	if err != ErrDuplicate {
		t.Fatalf("Expected error adding duplicate link")
	}

	// Test getting all links
	links, err := database.GetAllLinks()
	if err != nil {
		t.Fatalf("Failed to get links: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("Expected 2 link, got %d", len(links))
	}
	if links[0].URL != url {
		t.Errorf("Expected URL %s, got %s", url, links[0].URL)
	}
	if links[0].Title != title {
		t.Errorf("Expected title '%s', got '%s'", title, links[0].Title)
	}
	if links[0].Title != title {
		t.Errorf("Expected description '%s', got '%s'", description, links[0].Description)
	}
	if links[0].AddedAt.IsZero() {
		t.Errorf("Expected non-zero AddedAt")
	}
	if links[1].URL != url2 {
		t.Errorf("Expected URL %s, got %s", url2, links[1].URL)
	}
	if links[1].Title != title2 {
		t.Errorf("Expected title '%s', got '%s'", title2, links[1].Title)
	}
	if links[1].Title != title2 {
		t.Errorf("Expected description '%s', got '%s'", description2, links[1].Description)
	}
	if links[1].AddedAt.IsZero() {
		t.Errorf("Expected non-zero AddedAt")
	}

	// Test search
	linksSearch, err := database.Search("peculiar")
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}
	if len(linksSearch) != 1 {
		t.Fatalf("Expected 1 links, got %d", len(linksSearch))
	}
	if linksSearch[0].URL != url {
		t.Errorf("Expected single URL %s, got %s", url, linksSearch[0].URL)
	}
	if linksSearch[0].Title != title {
		t.Errorf("Expected single title '%s', got '%s'", title, linksSearch[0].Title)
	}
	if linksSearch[0].Description != description {
		t.Errorf("Expected single description '%s', got '%s'", description, linksSearch[0].Description)
	}
	if linksSearch[0].AddedAt.IsZero() {
		t.Errorf("Expected single non-zero AddedAt")
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
		t.Errorf("Expected single title '%s', got '%s'", title, link.Title)
	}
	if link.Description != description {
		t.Errorf("Expected single description '%s', got '%s'", description, link.Description)
	}
	if link.AddedAt.IsZero() {
		t.Errorf("Expected single non-zero AddedAt")
	}

	// Test non-existent link
	_, err = database.GetLink(99999)
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound for fetching non-existent link, got: %v", err)
	}

	// Test updating a link
	err = database.UpdateLink(id, "Updated title")
	if err != nil {
		t.Fatalf("Failed to update link: %v", err)
	}
	link, err = database.GetLink(id)
	if err != nil {
		t.Errorf("Failed to get updated link: %v", err)
	}
	if link.Title != "Updated title" {
		t.Errorf("Expected updated title '%s', got '%s'", "Updated title", link.Title)
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
	if len(links) != 1 {
		t.Fatalf("Expected 1 links after deletion, got %d", len(links))
	}
}
