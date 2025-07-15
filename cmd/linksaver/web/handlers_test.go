package web

import (
	"bytes"
	"github.com/mikaelstaldal/linksaver/cmd/linksaver/db"
	"golang.org/x/crypto/bcrypt"
	"io"
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
	testUsername := "test username"
	testPassword := "test password"

	// Initialize the database
	database, err := db.InitDB(dbFile)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	_, err = database.AddLink(testUrl, testTitle, testDescription, nil)
	if err != nil {
		t.Fatalf("Failed to add link: %v", err)
	}

	t.Cleanup(func() {
		_ = database.Close()
		_ = os.Remove(dbFile)
	})

	usernameBcryptHash, err := bcrypt.GenerateFromPassword([]byte(testUsername), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("Failed to hash username: %v", err)
	}
	t.Logf("Username: %v", string(usernameBcryptHash))

	passwordBcryptHash, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	handler := NewHandlers("../../..", database, "", usernameBcryptHash, passwordBcryptHash).Routes()

	t.Run("get all links success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.SetBasicAuth(testUsername, testPassword)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		result := rr.Result()
		body, err := io.ReadAll(result.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}
		_ = result.Body.Close()

		if status := result.StatusCode; status != http.StatusOK {
			t.Errorf("Handlers returned wrong status code: got %v want %v", status, http.StatusOK)
		}

		if !bytes.Contains(body, []byte(testUrl)) {
			t.Errorf("Response doesn't contain the expected link URL\n%s", rr.Body.String())
		}
		if !bytes.Contains(body, []byte(testTitle)) {
			t.Errorf("Response doesn't contain the expected link title\n%s", rr.Body.String())
		}
		if !bytes.Contains(body, []byte(testDescription)) {
			t.Errorf("Response doesn't contain the expected link description\n%s", rr.Body.String())
		}
		if !bytes.Contains(body, []byte(time.Now().Format("2006-01-02 "))) {
			t.Errorf("Response doesn't contain the expected date\n%s", rr.Body.String())
		}
	})

	t.Run("search success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?s=test", nil)
		req.SetBasicAuth(testUsername, testPassword)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		result := rr.Result()
		body, err := io.ReadAll(result.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}
		_ = result.Body.Close()

		if status := result.StatusCode; status != http.StatusOK {
			t.Errorf("Handlers returned wrong status code: got %v want %v", status, http.StatusOK)
		}

		if !bytes.Contains(body, []byte(testUrl)) {
			t.Errorf("Response doesn't contain the expected link URL\n%s", rr.Body.String())
		}
		if !bytes.Contains(body, []byte(testTitle)) {
			t.Errorf("Response doesn't contain the expected link title\n%s", rr.Body.String())
		}
		if !bytes.Contains(body, []byte(testDescription)) {
			t.Errorf("Response doesn't contain the expected link description\n%s", rr.Body.String())
		}
		if !bytes.Contains(body, []byte(time.Now().Format("2006-01-02 "))) {
			t.Errorf("Response doesn't contain the expected date\n%s", rr.Body.String())
		}
	})

	t.Run("get single link success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/1", nil)
		req.SetBasicAuth(testUsername, testPassword)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		result := rr.Result()
		body, err := io.ReadAll(result.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}
		_ = result.Body.Close()

		if status := result.StatusCode; status != http.StatusOK {
			t.Errorf("Handlers returned wrong status code: got %v want %v", status, http.StatusOK)
		}

		if !bytes.Contains(body, []byte(testUrl)) {
			t.Errorf("Response doesn't contain the expected link URL\n%s", rr.Body.String())
		}
		if !bytes.Contains(body, []byte(testTitle)) {
			t.Errorf("Response doesn't contain expected title\n%s", rr.Body.String())
		}
		if !bytes.Contains(body, []byte(testDescription)) {
			t.Errorf("Response doesn't contain the expected link description\n%s", rr.Body.String())
		}
	})

	t.Run("get single link invalid id", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/invalid", nil)
		req.SetBasicAuth(testUsername, testPassword)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		result := rr.Result()
		_, err := io.Copy(io.Discard, result.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}
		_ = result.Body.Close()

		if status := result.StatusCode; status != http.StatusBadRequest {
			t.Errorf("Handlers returned wrong status code: got %v want %v", status, http.StatusBadRequest)
		}
	})

	t.Run("get single link not found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/999", nil)
		req.SetBasicAuth(testUsername, testPassword)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		result := rr.Result()
		_, err := io.Copy(io.Discard, result.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}
		_ = result.Body.Close()

		if status := result.StatusCode; status != http.StatusNotFound {
			t.Errorf("Handlers returned wrong status code: got %v want %v", status, http.StatusNotFound)
		}
	})

	t.Run("delete link success", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/1", nil)
		req.SetBasicAuth(testUsername, testPassword)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		result := rr.Result()
		_, err := io.Copy(io.Discard, result.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}
		_ = result.Body.Close()

		if status := result.StatusCode; status != http.StatusOK {
			t.Errorf("Handlers returned wrong status code: got %v want %v", status, http.StatusOK)
		}

		// Verify link was deleted
		_, err = database.GetLink(1)
		if err == nil {
			t.Error("Link should have been deleted")
		}
	})

	t.Run("delete link invalid id", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/invalid", nil)
		req.SetBasicAuth(testUsername, testPassword)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		result := rr.Result()
		_, err := io.Copy(io.Discard, result.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}
		_ = result.Body.Close()

		if status := result.StatusCode; status != http.StatusBadRequest {
			t.Errorf("Handlers returned wrong status code: got %v want %v", status, http.StatusBadRequest)
		}
	})

	t.Run("delete link not found", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/999", nil)
		req.SetBasicAuth(testUsername, testPassword)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		result := rr.Result()
		_, err := io.Copy(io.Discard, result.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}
		_ = result.Body.Close()

		if status := result.StatusCode; status != http.StatusNotFound {
			t.Errorf("Handlers returned wrong status code: got %v want %v", status, http.StatusNotFound)
		}
	})

	t.Run("unauthorized", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		result := rr.Result()
		_, err := io.Copy(io.Discard, result.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}
		_ = result.Body.Close()

		if status := result.StatusCode; status != http.StatusUnauthorized {
			t.Errorf("Handlers returned wrong status code: got %v want %v", status, http.StatusUnauthorized)
		}
	})

}

func Test_extractTitleAndDescriptionAndBodyFromURL(t *testing.T) {
	tests := []struct {
		name         string
		contentType  string
		returnedBody []byte
		title        string
		description  string
		body         []byte
		wantErr      bool
	}{
		{
			name:         "valid HTML page",
			contentType:  "text/html",
			returnedBody: []byte("<html><head><title>Example Domain</title><meta name='description' content='This domain is for use in illustrative examples in documents.'></head><body>\n<div>\n<h1>Some header</h1>\n</div>\n</body></html>"),
			title:        "Example Domain",
			description:  "This domain is for use in illustrative examples in documents.",
			body:         []byte("<body>\n<div>\n<h1>Some header</h1>\n</div>\n</body>"),
			wantErr:      false,
		},
		{
			name:         "not HTML content",
			contentType:  "image/jpeg",
			returnedBody: []byte("binary data"),
			title:        "",
			description:  "",
			body:         nil,
			wantErr:      true,
		},
		{
			name:         "no title found",
			contentType:  "text/html",
			returnedBody: []byte("<html><head><meta name='description' content='This domain is for use in illustrative examples in documents.'></head><body>\n<div>\n<h1>Some header</h1>\n</div>\n</body></html"),
			title:        "",
			description:  "",
			body:         nil,
			wantErr:      true,
		},
		{
			name:         "very long title",
			contentType:  "text/html",
			returnedBody: []byte("<html><head><title>" + strings.Repeat("a", maxTitleLength+100) + "</title><meta name='description' content='This domain is for use in illustrative examples in documents.'></head><body>\n<div>\n<h1>Some header</h1>\n</div>\n</body></html"),
			title:        strings.Repeat("a", maxTitleLength) + "...",
			description:  "This domain is for use in illustrative examples in documents.",
			body:         []byte("<body>\n<div>\n<h1>Some header</h1>\n</div>\n</body>"),
			wantErr:      false,
		},
		{
			name:         "very long description",
			contentType:  "text/html",
			returnedBody: []byte("<html><head><title>Example Domain</title><meta name='description' content='" + strings.Repeat("b", maxDescriptionLength+100) + "'></head><body>\n<div>\n<h1>Some header</h1>\n</div>\n</body></html"),
			title:        "Example Domain",
			description:  strings.Repeat("b", maxDescriptionLength) + "...",
			body:         []byte("<body>\n<div>\n<h1>Some header</h1>\n</div>\n</body>"),
			wantErr:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(tt.returnedBody)
			}))
			defer server.Close()

			title, description, body, err := extractTitleAndDescriptionAndBodyFromURL(server.URL)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractTitleAndDescriptionAndBodyFromURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if title != tt.title {
				t.Errorf("extractTitleAndDescriptionAndBodyFromURL() title = '%v', title '%v'", title, tt.title)
			}
			if description != tt.description {
				t.Errorf("extractTitleAndDescriptionAndBodyFromURL() description = '%v', description '%v'", description, tt.description)
			}
			if !bytes.HasPrefix(body, tt.body) {
				t.Errorf("extractTitleAndDescriptionAndBodyFromURL() body = '%v', body '%v'", string(body), string(tt.body))
			}
		})
	}
}
