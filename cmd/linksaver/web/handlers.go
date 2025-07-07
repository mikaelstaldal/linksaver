package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/mikaelstaldal/linksaver/cmd/linksaver/db"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

// Handlers holds dependencies for the handlers
type Handlers struct {
	executableDir  string
	database       *db.DB
	screenshotsDir string
	templates      *template.Template
	browserContext context.Context
}

// NewHandlers creates a new Handlers
func NewHandlers(executableDir string, database *db.DB, screenshotsDir string) *Handlers {
	templates := template.Must(template.New("").Funcs(template.FuncMap{"screenshotFilename": screenshotFilename}).
		ParseGlob(filepath.Join(executableDir, "ui/templates/*.html")))

	dockerURL := "wss://localhost:9222"
	allocatorContext, _ := chromedp.NewRemoteAllocator(context.Background(), dockerURL)
	browserContext, _ := chromedp.NewContext(allocatorContext)

	return &Handlers{
		executableDir:  executableDir,
		database:       database,
		screenshotsDir: screenshotsDir,
		templates:      templates,
		browserContext: browserContext,
	}
}

func (h *Handlers) Routes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.Handle("GET /static/", http.StripPrefix("/static", http.FileServer(http.Dir(filepath.Join(h.executableDir, "ui/static")))))
	mux.Handle("GET /screenshots/", http.StripPrefix("/screenshots", http.FileServer(http.Dir(h.screenshotsDir))))

	mux.HandleFunc("GET /{$}", h.ListLinks)
	mux.HandleFunc("POST /{$}", h.AddLink)
	mux.HandleFunc("GET /{id}", h.GetLink)
	mux.HandleFunc("DELETE /{id}", h.DeleteLink)

	return mux
}

type Link struct {
	ID          int64
	URL         string
	Title       string
	Description string
	AddedAt     time.Time
	Screenshot  string
}

// ListLinks handles the request to list all links
func (h *Handlers) ListLinks(w http.ResponseWriter, r *http.Request) {
	h.listLinks(w, r, http.StatusOK)
}

// AddLink handles the request to add a new link
func (h *Handlers) AddLink(w http.ResponseWriter, r *http.Request) {
	// Parse form data
	if err := r.ParseForm(); err != nil {
		sendError(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	urlString := r.FormValue("url")

	if urlString == "" {
		sendError(w, "URL is required", http.StatusBadRequest)
		return
	}

	// Parse and validate URL
	parsedURL, err := url.Parse(urlString)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") ||
		parsedURL.Host == "" || strings.HasPrefix(strings.ToLower(parsedURL.Host), "localhost") ||
		strings.HasPrefix(parsedURL.Host, "127.") ||
		strings.HasPrefix(parsedURL.Host, "0.") ||
		strings.HasPrefix(parsedURL.Host, "::1") {
		sendError(w, "Invalid URL format. Must be a valid HTTP/HTTPS URL", http.StatusBadRequest)
		return
	}
	urlToSave := parsedURL.String()

	// Extract title from the URL
	title, description, body, screenshot, err := h.extractTitleAndDescriptionAndBodyAndScreenshotFromURL(urlToSave)
	if err != nil {
		sendError(w, fmt.Sprintf("Failed to load URL: %v", err), http.StatusBadRequest)
		return
	}

	id, err := h.database.AddLink(urlToSave, title, description, body)
	if err != nil {
		if errors.Is(err, db.ErrDuplicate) {
			sendError(w, "URL already exists", http.StatusConflict)
		} else {
			sendError(w, fmt.Sprintf("Failed to add link: %v", err), http.StatusInternalServerError)
		}
		return
	}

	if err = h.saveScreenshot(urlToSave, screenshot); err != nil {
		sendError(w, fmt.Sprintf("Failed to save screenshot: %v", err), http.StatusInternalServerError)
	}

	w.Header().Set("Location", fmt.Sprintf("/%v", id))
	h.listLinks(w, r, http.StatusCreated)
}

func (h *Handlers) extractTitleAndDescriptionAndBodyAndScreenshotFromURL(url string) (string, string, string, []byte, error) {
	response, err := chromedp.RunResponse(h.browserContext,
		chromedp.Navigate(url),
	)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	if response.Status >= 400 {
		return "", "", "", nil, fmt.Errorf("failed to fetch URL: %v %v", response.Status, response.StatusText)
	}

	var title string
	err = chromedp.Run(h.browserContext,
		chromedp.Title(&title),
	)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("failed to extract title: %w", err)
	}
	title = strings.TrimSpace(title)

	var description string
	err = chromedp.Run(h.browserContext,
		chromedp.Evaluate(`document.querySelector("head meta[name='description']").content`, &description),
	)
	if err != nil {
		description = ""
	}
	description = strings.TrimSpace(description)

	var body string
	err = chromedp.Run(h.browserContext,
		chromedp.OuterHTML(`body`, &body),
	)
	if err != nil {
		log.Printf("failed to extract body: %v", err)
		body = ""
	}
	body = strings.TrimSpace(body)

	var screenshot []byte
	err = chromedp.Run(h.browserContext,
		chromedp.EmulateViewport(800, 600),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			screenshot, err = page.CaptureScreenshot().
				WithFromSurface(true).
				WithFormat(page.CaptureScreenshotFormatPng).
				WithQuality(100).
				Do(ctx)
			if err != nil {
				return err
			}
			return nil
		}),
	)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("failed to take screenshot: %w", err)
	}

	if title == "" {
		return "", "", "", nil, fmt.Errorf("no title found in HTML")
	}

	if len(title) > 250 {
		title = title[:250] + "..."
	}

	if len(description) > 1020 {
		description = description[:1020] + "..."
	}

	if len(body) > 100000 {
		body = body[:100000]
	}

	return title, description, body, screenshot, nil
}

func (h *Handlers) saveScreenshot(urlString string, screenshot []byte) error {
	filename := screenshotFilename(urlString)
	path := filepath.Join(h.screenshotsDir, filename)

	if err := os.WriteFile(path, screenshot, 0644); err != nil {
		return fmt.Errorf("failed to write screenshot file: %w", err)
	}

	return nil
}

// GetLink gets a single link
func (h *Handlers) GetLink(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		sendError(w, "Invalid ID: "+err.Error(), http.StatusBadRequest)
		return
	}

	dbLink, err := h.database.GetLink(id)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			sendError(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		} else {
			sendError(w, fmt.Sprintf("Failed to get link: %v\n", err), http.StatusInternalServerError)
		}
		return
	}

	h.render(w, "link", dbLink, http.StatusOK)
}

// DeleteLink handles the request to delete a link
func (h *Handlers) DeleteLink(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		sendError(w, "Invalid ID: "+err.Error(), http.StatusBadRequest)
		return
	}

	err = h.database.DeleteLink(id)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			sendError(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		} else {
			sendError(w, fmt.Sprintf("Failed delete link: %v\n", err), http.StatusInternalServerError)
		}
		return
	}

	screenshotPath := filepath.Join(h.screenshotsDir, fmt.Sprintf("%d.png", id))
	if err := os.Remove(screenshotPath); err != nil && !os.IsNotExist(err) {
		sendError(w, fmt.Sprintf("Failed delete screenshot: %v\n", err), http.StatusInternalServerError)
	}
}

func (h *Handlers) listLinks(w http.ResponseWriter, r *http.Request, status int) {
	search := r.URL.Query().Get("s")
	var dbLinks []db.Link
	var err error
	if search != "" {
		dbLinks, err = h.database.Search(search)
		if err != nil {
			sendError(w, fmt.Sprintf("Failed to search: %v\n", err), http.StatusInternalServerError)
			return
		}
	} else {
		dbLinks, err = h.database.GetAllLinks()
		if err != nil {
			sendError(w, fmt.Sprintf("Failed to get links: %v\n", err), http.StatusInternalServerError)
			return
		}
	}

	// Format dates in the required format
	links := make([]db.Link, 0, len(dbLinks))
	for _, dbLink := range dbLinks {
		links = append(links, dbLink)
	}

	data := struct {
		Search string
		Links  []db.Link
	}{
		Search: search,
		Links:  links,
	}
	var templateName string
	if r.Header.Get("HX-Request") == "true" {
		templateName = "links"
	} else {
		templateName = "index.html"
	}
	h.render(w, templateName, data, status)
}

func (h *Handlers) render(w http.ResponseWriter, name string, data any, status int) {
	buf := new(bytes.Buffer)
	err := h.templates.ExecuteTemplate(buf, name, data)
	if err != nil {
		sendError(w, fmt.Sprintf("Failed to render %s: %v\n", name, err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

func sendError(w http.ResponseWriter, errorMessage string, status int) {
	var message string
	if status >= 500 {
		log.Println(errorMessage + "\n" + string(debug.Stack()))
		message = http.StatusText(status)
	} else {
		message = errorMessage
	}
	w.WriteHeader(status)
	_, _ = fmt.Fprintln(w, message)
}

func screenshotFilename(urlString string) string {
	hash := sha256.Sum256([]byte(urlString))
	return hex.EncodeToString(hash[:]) + ".png"
}
