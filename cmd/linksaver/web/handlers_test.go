package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/mikaelstaldal/linksaver/cmd/linksaver/db"
	"golang.org/x/crypto/bcrypt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestHandlers(t *testing.T) {
	// Use a temporary database file for testing
	dbFile := "test_handlers.database"

	testTitle := "Test Title"
	testDescription := "Test Description"
	testUsername := "test username"
	testPassword := "test password"

	// Initialize the database
	database, err := db.InitDB(dbFile)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
		_ = os.Remove(dbFile)
	})

	usernameBcryptHash, err := bcrypt.GenerateFromPassword([]byte(testUsername), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("Failed to hash username: %v", err)
	}

	passwordBcryptHash, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	handler := NewHandlers("../../..", database, "", usernameBcryptHash, passwordBcryptHash).Routes()

	// Create a mock HTTP server to simulate a valid URL
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf("<html><head><title>%s</title><meta name='description' content='%s'></head><body>Some body</body></html>", testTitle, testDescription)))
	}))
	defer mockServer.Close()

	var linkId int64

	t.Run("add link success", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/", strings.NewReader("url="+mockServer.URL))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth(testUsername, testPassword)
		response, body := testRequest(t, handler, req)

		if status := response.StatusCode; status != http.StatusCreated {
			t.Errorf("Handlers returned wrong status code: got %v want %v", status, http.StatusCreated)
		}

		locationHeader := response.Header.Get("Location")
		if linkIdString, found := strings.CutPrefix(locationHeader, "/"); !found {
			t.Errorf("Response Location header doesn't has correct format: '%s'", locationHeader)
		} else {
			if linkId, err = strconv.ParseInt(linkIdString, 10, 64); err != nil {
				t.Errorf("Failed to convert link ID: %v", err)
			}
		}

		if !bytes.Contains(body, []byte(mockServer.URL)) {
			t.Errorf("Response doesn't contain the expected link URL\n%s", string(body))
		}
		if !bytes.Contains(body, []byte(testTitle)) {
			t.Errorf("Response doesn't contain the expected link title\n%s", string(body))
		}
		if !bytes.Contains(body, []byte(testDescription)) {
			t.Errorf("Response doesn't contain the expected link description\n%s", string(body))
		}
	})

	t.Run("add link missing url", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/", strings.NewReader(""))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth(testUsername, testPassword)
		response, _ := testRequest(t, handler, req)

		if status := response.StatusCode; status != http.StatusBadRequest {
			t.Errorf("Handlers returned wrong status code: got %v want %v", status, http.StatusBadRequest)
		}
	})

	t.Run("add link invalid url", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/", strings.NewReader("url=invalid-url"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth(testUsername, testPassword)
		response, _ := testRequest(t, handler, req)

		if status := response.StatusCode; status != http.StatusBadRequest {
			t.Errorf("Handlers returned wrong status code: got %v want %v", status, http.StatusBadRequest)
		}
	})

	t.Run("get all links success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.SetBasicAuth(testUsername, testPassword)
		response, body := testRequest(t, handler, req)

		if status := response.StatusCode; status != http.StatusOK {
			t.Errorf("Handlers returned wrong status code: got %v want %v", status, http.StatusOK)
		}

		if !bytes.Contains(body, []byte(mockServer.URL)) {
			t.Errorf("Response doesn't contain the expected link URL\n%s", string(body))
		}
		if !bytes.Contains(body, []byte(testTitle)) {
			t.Errorf("Response doesn't contain the expected link title\n%s", string(body))
		}
		if !bytes.Contains(body, []byte(testDescription)) {
			t.Errorf("Response doesn't contain the expected link description\n%s", string(body))
		}
		if !bytes.Contains(body, []byte(time.Now().Format("2006-01-02 "))) {
			t.Errorf("Response doesn't contain the expected date\n%s", string(body))
		}
	})

	t.Run("get all links as JSON", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept", "application/json")
		req.SetBasicAuth(testUsername, testPassword)
		response, body := testRequest(t, handler, req)

		if status := response.StatusCode; status != http.StatusOK {
			t.Errorf("Handlers returned wrong status code: got %v want %v", status, http.StatusOK)
		}

		if contentType := response.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
			t.Errorf("Wrong Content-Type: %s", contentType)
		}

		var data []db.Link
		if err := json.Unmarshal(body, &data); err != nil {
			t.Errorf("Response doesn't contain the expected JSON\n%s", string(body))
		}
		if len(data) != 1 {
			t.Errorf("Wrong length of JSON response: %d", len(data))
		}
		if data[0].URL != mockServer.URL {
			t.Errorf("Response doesn't contain the expected link URL\n%s", data[0].URL)
		}
		if data[0].Title != testTitle {
			t.Errorf("Response doesn't contain the expected link title\n%s", data[0].Title)
		}
		if data[0].Description != testDescription {
			t.Errorf("Response doesn't contain the expected link description\n%s", data[0].Description)
		}
	})

	t.Run("search success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?s=test", nil)
		req.SetBasicAuth(testUsername, testPassword)
		response, body := testRequest(t, handler, req)

		if status := response.StatusCode; status != http.StatusOK {
			t.Errorf("Handlers returned wrong status code: got %v want %v", status, http.StatusOK)
		}

		if !bytes.Contains(body, []byte(mockServer.URL)) {
			t.Errorf("Response doesn't contain the expected link URL\n%s", string(body))
		}
		if !bytes.Contains(body, []byte(testTitle)) {
			t.Errorf("Response doesn't contain the expected link title\n%s", string(body))
		}
		if !bytes.Contains(body, []byte(testDescription)) {
			t.Errorf("Response doesn't contain the expected link description\n%s", string(body))
		}
		if !bytes.Contains(body, []byte(time.Now().Format("2006-01-02 "))) {
			t.Errorf("Response doesn't contain the expected date\n%s", string(body))
		}
	})

	t.Run("get single link success", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/%d", linkId), nil)
		req.SetBasicAuth(testUsername, testPassword)
		response, body := testRequest(t, handler, req)

		if status := response.StatusCode; status != http.StatusOK {
			t.Errorf("Handlers returned wrong status code: got %v want %v", status, http.StatusOK)
		}

		if !bytes.Contains(body, []byte(mockServer.URL)) {
			t.Errorf("Response doesn't contain the expected link URL\n%s", string(body))
		}
		if !bytes.Contains(body, []byte(testTitle)) {
			t.Errorf("Response doesn't contain expected title\n%s", string(body))
		}
		if !bytes.Contains(body, []byte(testDescription)) {
			t.Errorf("Response doesn't contain the expected link description\n%s", string(body))
		}
	})

	t.Run("get single link as JSON", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/%d", linkId), nil)
		req.Header.Set("Accept", "application/json")
		req.SetBasicAuth(testUsername, testPassword)
		response, body := testRequest(t, handler, req)

		if status := response.StatusCode; status != http.StatusOK {
			t.Errorf("Handlers returned wrong status code: got %v want %v", status, http.StatusOK)
		}

		if contentType := response.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
			t.Errorf("Wrong Content-Type: %s", contentType)
		}

		var data db.Link
		if err := json.Unmarshal(body, &data); err != nil {
			t.Errorf("Response doesn't contain the expected JSON\n%s", string(body))
		}
		if data.URL != mockServer.URL {
			t.Errorf("Response doesn't contain the expected link URL\n%s", data.URL)
		}
		if data.Title != testTitle {
			t.Errorf("Response doesn't contain the expected link title\n%s", data.Title)
		}
		if data.Description != testDescription {
			t.Errorf("Response doesn't contain the expected link description\n%s", data.Description)
		}
	})

	t.Run("get single link invalid id", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/invalid", nil)
		req.SetBasicAuth(testUsername, testPassword)
		response, _ := testRequest(t, handler, req)

		if status := response.StatusCode; status != http.StatusBadRequest {
			t.Errorf("Handlers returned wrong status code: got %v want %v", status, http.StatusBadRequest)
		}
	})

	t.Run("get single link not found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/999", nil)
		req.SetBasicAuth(testUsername, testPassword)
		response, _ := testRequest(t, handler, req)

		if status := response.StatusCode; status != http.StatusNotFound {
			t.Errorf("Handlers returned wrong status code: got %v want %v", status, http.StatusNotFound)
		}
	})

	t.Run("patch link success", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", fmt.Sprintf("/%d", linkId), strings.NewReader("title=Updated Title"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth(testUsername, testPassword)
		response, body := testRequest(t, handler, req)

		if status := response.StatusCode; status != http.StatusOK {
			t.Errorf("Handlers returned wrong status code: got %v want %v", status, http.StatusOK)
		}

		if !bytes.Contains(body, []byte("Updated Title")) {
			t.Errorf("Response doesn't contain the updated title\n%s", string(body))
		}

		// Verify the link was actually updated in the database
		updatedLink, err := database.GetLink(linkId)
		if err != nil {
			t.Fatalf("Failed to get updated link: %v", err)
		}
		if updatedLink.Title != "Updated Title" {
			t.Errorf("Link title was not updated in database: got %v want %v", updatedLink.Title, "Updated Title")
		}
	})

	t.Run("patch link invalid id", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/invalid", strings.NewReader("title=Updated Title"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth(testUsername, testPassword)
		response, _ := testRequest(t, handler, req)

		if status := response.StatusCode; status != http.StatusBadRequest {
			t.Errorf("Handlers returned wrong status code: got %v want %v", status, http.StatusBadRequest)
		}
	})

	t.Run("patch link not found", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/999", strings.NewReader("title=Updated Title"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth(testUsername, testPassword)
		response, _ := testRequest(t, handler, req)

		if status := response.StatusCode; status != http.StatusNotFound {
			t.Errorf("Handlers returned wrong status code: got %v want %v", status, http.StatusNotFound)
		}
	})

	t.Run("patch link missing title", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", fmt.Sprintf("/%d", linkId), strings.NewReader(""))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth(testUsername, testPassword)
		response, _ := testRequest(t, handler, req)

		if status := response.StatusCode; status != http.StatusBadRequest {
			t.Errorf("Handlers returned wrong status code: got %v want %v", status, http.StatusBadRequest)
		}
	})

	t.Run("delete link success", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", fmt.Sprintf("/%d", linkId), nil)
		req.SetBasicAuth(testUsername, testPassword)
		response, _ := testRequest(t, handler, req)

		if status := response.StatusCode; status != http.StatusOK {
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
		response, _ := testRequest(t, handler, req)

		if status := response.StatusCode; status != http.StatusBadRequest {
			t.Errorf("Handlers returned wrong status code: got %v want %v", status, http.StatusBadRequest)
		}
	})

	t.Run("delete link not found", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/999", nil)
		req.SetBasicAuth(testUsername, testPassword)
		response, _ := testRequest(t, handler, req)

		if status := response.StatusCode; status != http.StatusNotFound {
			t.Errorf("Handlers returned wrong status code: got %v want %v", status, http.StatusNotFound)
		}
	})

	t.Run("unauthorized", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		response, _ := testRequest(t, handler, req)

		if status := response.StatusCode; status != http.StatusUnauthorized {
			t.Errorf("Handlers returned wrong status code: got %v want %v", status, http.StatusUnauthorized)
		}
	})
}

func testRequest(t *testing.T, handler http.Handler, req *http.Request) (*http.Response, []byte) {
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	result := rr.Result()
	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}
	_ = result.Body.Close()
	return result, body
}

func Test_extractTitleAndDescriptionAndBodyFromURL(t *testing.T) {
	handlers := NewHandlers("../../..", nil, "", nil, nil)

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

			title, description, body, err := handlers.extractTitleAndDescriptionAndBodyFromURL(server.URL)
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
