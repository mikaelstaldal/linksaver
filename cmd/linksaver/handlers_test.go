package main

import (
	"github.com/mikaelstaldal/linksaver/cmd/linksaver/db"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestHandlers(t *testing.T) {
	// Use a temporary database file for testing
	dbFile := "test_handlers.database"

	testUrl := "https://www.some-test-url.com"
	testTitle := "Test Title"
	testDescription := "Test Description"

	// Initialize the database
	database, err := db.InitDB(dbFile)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	_, err = database.AddLink(testUrl, testTitle, testDescription, "")
	if err != nil {
		t.Fatalf("Failed to add link: %v", err)
	}

	t.Cleanup(func() {
		_ = database.Close()
		_ = os.Remove(dbFile)
	})

	handler := NewHandler(database).Routes()

	t.Run("get all links success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}

		if !strings.Contains(rr.Body.String(), testUrl) {
			t.Errorf("Response doesn't contain the expected link URL\n%s", rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), testTitle) {
			t.Errorf("Response doesn't contain the expected link title\n%s", rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), testDescription) {
			t.Errorf("Response doesn't contain the expected link description\n%s", rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), time.Now().Format("2006-01-02 ")) {
			t.Errorf("Response doesn't contain the expected date\n%s", rr.Body.String())
		}
	})

	t.Run("search success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?s=test", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}

		if !strings.Contains(rr.Body.String(), testUrl) {
			t.Errorf("Response doesn't contain the expected link URL\n%s", rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), testTitle) {
			t.Errorf("Response doesn't contain the expected link title\n%s", rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), testDescription) {
			t.Errorf("Response doesn't contain the expected link description\n%s", rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), time.Now().Format("2006-01-02 ")) {
			t.Errorf("Response doesn't contain the expected date\n%s", rr.Body.String())
		}
	})

	t.Run("get single link success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/1", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}

		if !strings.Contains(rr.Body.String(), testUrl) {
			t.Errorf("Response doesn't contain the expected link URL\n%s", rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), testTitle) {
			t.Errorf("Response doesn't contain expected title\n%s", rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), testDescription) {
			t.Errorf("Response doesn't contain the expected link description\n%s", rr.Body.String())
		}
	})

	t.Run("get single link invalid id", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/invalid", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusBadRequest {
			t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
		}
	})

	t.Run("get single link not found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/999", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusNotFound {
			t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusNotFound)
		}
	})

	t.Run("delete link success", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/1", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}

		// Verify link was deleted
		_, err := database.GetLink(1)
		if err == nil {
			t.Error("Link should have been deleted")
		}
	})

	t.Run("delete link invalid id", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/invalid", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusBadRequest {
			t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
		}
	})

	t.Run("delete link not found", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/999", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusNotFound {
			t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusNotFound)
		}
	})
}
