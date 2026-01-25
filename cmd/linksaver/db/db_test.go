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
		if database != nil {
			_ = database.Close()
		}
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
		t.Fatalf("Got %d, expected positive ID", id)
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
		t.Fatalf("Got %d, expected positive ID", id)
	}
	if id2 == id {
		t.Fatalf("Expected different id")
	}

	// Test adding a link without a body
	url3 := "https://empty.com"
	title3 := "PDF document"
	description3 := "application/pdf"
	id3, err := database.AddLink(url3, title3, description3, nil)
	if err != nil {
		t.Fatalf("Failed to add link 3: %v", err)
	}
	if id3 <= 0 {
		t.Fatalf("Got %d, expected positive ID", id)
	}
	if id3 == id || id3 == id2 {
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
	if len(links) != 3 {
		t.Fatalf("Got %d links, expected 3", len(links))
	}
	if links[0].URL != url {
		t.Errorf("Got URL %s, expected %s", links[0].URL, url)
	}
	if links[0].Title != title {
		t.Errorf("Got title '%s', expected '%s'", links[0].Title, title)
	}
	if links[0].Title != title {
		t.Errorf("Got description '%s', expected '%s'", links[0].Description, description)
	}
	if links[0].AddedAt.IsZero() {
		t.Errorf("Expected non-zero AddedAt")
	}
	if links[1].URL != url2 {
		t.Errorf("Got URL %s, expected %s", links[1].URL, url2)
	}
	if links[1].Title != title2 {
		t.Errorf("Got title '%s', expected '%s'", links[1].Title, title2)
	}
	if links[1].Title != title2 {
		t.Errorf("Got description '%s', expected '%s'", links[1].Description, description2)
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
		t.Fatalf("Got %d links, expected 1", len(linksSearch))
	}
	if linksSearch[0].URL != url {
		t.Errorf("Got URL %s, expected %s", linksSearch[0].URL, url)
	}
	if linksSearch[0].Title != title {
		t.Errorf("Got title '%s', expected '%s'", linksSearch[0].Title, title)
	}
	if linksSearch[0].Description != description {
		t.Errorf("Got description '%s', expected '%s'", linksSearch[0].Description, description)
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
		t.Errorf("Got URL %s, expected %s", link.URL, url)
	}
	if link.Title != title {
		t.Errorf("Got title '%s', expected '%s'", link.Title, title)
	}
	if link.Description != description {
		t.Errorf("Got description '%s', expected '%s'", link.Description, description)
	}
	if link.AddedAt.IsZero() {
		t.Errorf("Expected single non-zero AddedAt")
	}

	// Test non-existent link
	_, err = database.GetLink(99999)
	if err != ErrNotFound {
		t.Errorf("Got %v, expected ErrNotFound for fetching non-existent link", err)
	}

	// Test updating a link
	err = database.UpdateLink(id, "Updated title", "Updated description")
	if err != nil {
		t.Fatalf("Failed to update link: %v", err)
	}
	link, err = database.GetLink(id)
	if err != nil {
		t.Errorf("Failed to get updated link: %v", err)
	}
	if link.Title != "Updated title" {
		t.Errorf("Got title '%s', expected '%s'", link.Title, "Updated title")
	}
	if link.Description != "Updated description" {
		t.Errorf("Got description '%s', expected '%s'", link.Description, "Updated description")
	}

	// Test deleting a link
	err = database.DeleteLink(id)
	if err != nil {
		t.Fatalf("Failed to delete link: %v", err)
	}

	// Test deleting a non-existing link
	err = database.DeleteLink(9999)
	if err != ErrNotFound {
		t.Errorf("Got %v, expected ErrNotFound for deleting non-existent link", err)
	}

	// Verify the link was deleted
	links, err = database.GetAllLinks()
	if err != nil {
		t.Fatalf("Failed to get links after deletion: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("Got %d links after deletion, expected 2", len(links))
	}

	// Close the database
	err = database.Close()
	if err != nil {
		t.Fatalf("Failed to close database %v", err)
	}

	// Make the database file read-only
	err = os.Chmod(dbFile, 0400)
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to open the database again - should fail
	database, err = InitDB(dbFile)
	if err == nil {
		_ = database.Close()
		t.Fatalf("Unable to detect read-only database")
	}
}
