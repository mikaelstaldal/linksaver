package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/mikaelstaldal/linksaver/cmd/linksaver/db"
	"github.com/mikaelstaldal/linksaver/ui"
	"golang.org/x/net/html"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

const maxTitleLength = 250
const maxDescriptionLength = 1020
const maxBodyLength = 1000000

// Handlers holds dependencies for the handlers
type Handlers struct {
	executableDir      string
	database           *db.DB
	screenshotsDir     string
	templates          *template.Template
	browserContext     context.Context
	usernameBcryptHash []byte
	passwordBcryptHash []byte
}

// Create an HTTP client with improved configuration to handle various websites
var client = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		// Force HTTP/1.1 to avoid HTTP/2 issues with some websites
		ForceAttemptHTTP2: false,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
		// Set reasonable timeouts
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
	},
}

// NewHandlers creates a new Handlers
func NewHandlers(executableDir string, database *db.DB, screenshotsDir string, usernameBcryptHash, passwordBcryptHash []byte) *Handlers {
	templates := template.New("").Funcs(template.FuncMap{"screenshotFilename": screenshotFilename})

	templatesDir := filepath.Join(executableDir, "ui/templates")
	templateFiles, err := filepath.Glob(filepath.Join(templatesDir, "*.html"))
	if err != nil {
		log.Fatalf("Unable to glob templates: %v", err)
	}

	if len(templateFiles) > 0 {
		templates, err = templates.ParseFiles(templateFiles...)
	} else {
		templates, err = templates.ParseFS(ui.Files, "templates/*.html")
	}
	if err != nil {
		log.Fatalf("Unable to read templates: %v", err)
	}
	if templates == nil {
		log.Fatalf("No templates found")
	}

	var browserContext context.Context
	dockerURL := os.Getenv("CHROMEDP")
	if dockerURL != "" {
		if err := os.MkdirAll(screenshotsDir, 0755); err != nil {
			log.Fatalf("failed to create screenshots directory: %v", err)
		}

		allocatorContext, _ := chromedp.NewRemoteAllocator(context.Background(), dockerURL)
		browserContext, _ = chromedp.NewContext(allocatorContext)
	}

	return &Handlers{
		executableDir:      executableDir,
		database:           database,
		screenshotsDir:     screenshotsDir,
		templates:          templates,
		browserContext:     browserContext,
		usernameBcryptHash: usernameBcryptHash,
		passwordBcryptHash: passwordBcryptHash,
	}
}

func (h *Handlers) Routes() http.Handler {
	mux := http.NewServeMux()

	staticDir := filepath.Join(h.executableDir, "ui/static")
	staticFiles, err := filepath.Glob(filepath.Join(staticDir, "*"))
	if err != nil {
		log.Fatalf("Unable to glob static files: %v", err)
	}
	if len(staticFiles) > 0 {
		mux.Handle("GET /static/", http.StripPrefix("/static", http.FileServer(http.Dir(staticDir))))
	} else {
		mux.Handle("GET /static/", http.FileServerFS(ui.Files))
	}

	if h.browserContext != nil {
		mux.Handle("GET /screenshots/", http.StripPrefix("/screenshots", http.FileServer(http.Dir(h.screenshotsDir))))
	}

	mux.HandleFunc("GET /{$}", h.ListLinks)
	mux.HandleFunc("POST /{$}", h.AddLink)
	mux.HandleFunc("GET /{id}", h.GetLink)
	mux.HandleFunc("DELETE /{id}", h.DeleteLink)

	if h.usernameBcryptHash != nil && h.passwordBcryptHash != nil {
		return commonHeaders(h.basicAuth(mux))
	} else {
		return commonHeaders(mux)
	}
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
	if err := r.ParseForm(); err != nil {
		sendError(w, fmt.Sprintf("Failed to parse form: %v", err), http.StatusBadRequest)
		return
	}

	urlString := r.PostForm.Get("url")
	if urlString == "" {
		sendError(w, "URL is required", http.StatusBadRequest)
		return
	}

	// Parse and validate URL
	parsedURL, err := url.Parse(urlString)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || isPrivateOrLocalhost(parsedURL.Host) {
		sendError(w, "Invalid URL format. Must be a valid HTTP/HTTPS URL", http.StatusBadRequest)
		return
	}
	urlToSave := parsedURL.String()

	var title, description string
	var body []byte
	var screenshot []byte
	if h.browserContext != nil {
		title, description, body, screenshot, err = h.extractTitleAndDescriptionAndBodyAndScreenshotFromURL(urlToSave)
		if err != nil {
			sendError(w, fmt.Sprintf("Failed to load URL: %v", err), http.StatusBadRequest)
			return
		}
	} else {
		title, description, body, err = extractTitleAndDescriptionAndBodyFromURL(urlToSave)
		if err != nil {
			sendError(w, fmt.Sprintf("Failed to load URL: %v", err), http.StatusBadRequest)
			return
		}
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

	if screenshot != nil {
		if err = h.saveScreenshot(urlToSave, screenshot); err != nil {
			sendError(w, fmt.Sprintf("Failed to save screenshot: %v", err), http.StatusInternalServerError)
		}
	}

	w.Header().Set("Location", fmt.Sprintf("/%v", id))
	h.listLinks(w, r, http.StatusCreated)
}

func isPrivateOrLocalhost(host string) bool {
	ip := net.ParseIP(host)
	if ip != nil {
		return ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified()
	}
	// Handle hostname cases
	return host == "" || strings.HasPrefix(strings.ToLower(host), "localhost") ||
		strings.HasSuffix(strings.ToLower(host), ".localhost")
}

// extractTitleAndDescriptionAndBodyFromURL fetches the URL and extracts the page title from HTML
func extractTitleAndDescriptionAndBodyFromURL(url string) (string, string, []byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add browser-like headers to avoid being blocked by anti-bot measures
	req.Header.Set("User-Agent", "LinkSaver/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyLength))
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to fetch URL: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", "", nil, fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(strings.ToLower(contentType), "text/html") && !strings.HasPrefix(strings.ToLower(contentType), "application/xhtml+xml") {
		return "", "", nil, fmt.Errorf("content type is not HTML: %s", contentType)
	}

	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	title := strings.TrimSpace(extractTitle(doc))
	if title == "" {
		return "", "", nil, fmt.Errorf("no title found in HTML")
	}

	description := strings.TrimSpace(extractDescription(doc))

	if len(title) > maxTitleLength {
		title = title[:maxTitleLength] + "..."
	}

	if len(description) > maxDescriptionLength {
		description = description[:maxDescriptionLength] + "..."
	}

	return title, description, body, nil
}

// extractTitle recursively searches for the "title" element in the HTML tree
func extractTitle(n *html.Node) string {
	if n.Type == html.ElementNode && n.Data == "title" {
		// Found the title element, extract its text content
		return extractTextContent(n)
	}

	// Recursively search child nodes
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if title := extractTitle(c); title != "" {
			return title
		}
	}

	return ""
}

// extractTextContent extracts all text content from a node and its children
func extractTextContent(n *html.Node) string {
	var text strings.Builder

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			text.WriteString(c.Data)
		} else if c.Type == html.ElementNode {
			text.WriteString(extractTextContent(c))
		}
	}

	return text.String()
}

// extractDescription recursively searches for the "meta" element in the HTML tree
func extractDescription(n *html.Node) string {
	if n.Type == html.ElementNode && n.Data == "meta" && extractAttribute(n, "name") == "description" {
		return extractAttribute(n, "content")
	}

	// Recursively search child nodes
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if title := extractDescription(c); title != "" {
			return title
		}
	}

	return ""
}

func extractAttribute(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func (h *Handlers) extractTitleAndDescriptionAndBodyAndScreenshotFromURL(url string) (string, string, []byte, []byte, error) {
	response, err := chromedp.RunResponse(h.browserContext,
		chromedp.Navigate(url),
	)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	if response.Status >= 400 {
		return "", "", nil, nil, fmt.Errorf("failed to fetch URL: %v %v", response.Status, response.StatusText)
	}

	var title string
	err = chromedp.Run(h.browserContext,
		chromedp.Title(&title),
	)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("failed to extract title: %w", err)
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

	var body []byte
	err = chromedp.Run(h.browserContext,
		chromedp.JavascriptAttribute(`body`, "outerHTML", &body),
	)
	if err != nil {
		log.Printf("failed to extract body: %v", err)
	}

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
		return "", "", nil, nil, fmt.Errorf("failed to take screenshot: %w", err)
	}

	if title == "" {
		return "", "", nil, nil, fmt.Errorf("no title found in HTML")
	}

	if len(title) > maxTitleLength {
		title = title[:maxTitleLength] + "..." + "..."
	}

	if len(description) > maxDescriptionLength {
		description = description[:maxDescriptionLength] + "..."
	}

	if len(body) > maxBodyLength {
		body = body[:maxBodyLength]
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
		sendError(w, fmt.Sprintf("Invalid ID: %v", err), http.StatusBadRequest)
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

	if h.browserContext != nil {
		h.render(w, "link-with-screenshot", dbLink, http.StatusOK)
	} else {
		h.render(w, "link-without-screenshot", dbLink, http.StatusOK)
	}
}

// DeleteLink handles the request to delete a link
func (h *Handlers) DeleteLink(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		sendError(w, fmt.Sprintf("Invalid ID: %v", err), http.StatusBadRequest)
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
		log.Printf("Failed to delete screenshot: %v\n", err)
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

	data := struct {
		Search          string
		Links           []db.Link
		ShowScreenshots bool
	}{
		Search:          search,
		Links:           dbLinks,
		ShowScreenshots: h.browserContext != nil,
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
