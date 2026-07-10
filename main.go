package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const maxUploadBytes = 16 << 20

type server struct {
	logger *slog.Logger
}

type uploadedFile struct {
	Name string
	Size int64
	Type string
}

type processedFile struct {
	uploadedFile
	Updates []fieldUpdate
	Error   string
}

func main() {
	port := env("PORT", "8080")

	s := &server{
		logger: slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.HandleFunc("GET /", s.handleHome)
	mux.HandleFunc("GET /.well-known/agility-app.json", s.handleAppDefinition)
	mux.HandleFunc("GET /install", s.handleInstall)
	mux.HandleFunc("GET /home-dashboard", s.handleHomeDashboard)
	mux.HandleFunc("GET /content-dashboard", s.handleContentDashboard)
	mux.HandleFunc("GET /pages-dashboard", s.handlePagesDashboard)
	mux.HandleFunc("GET /content-item-sidebar", s.handleContentItemSidebar)
	mux.HandleFunc("GET /content-list-sidebar", s.handleContentListSidebar)
	mux.HandleFunc("GET /page-sidebar", s.handlePageSidebar)
	mux.HandleFunc("POST /content-item-sidebar/upload", s.handleUpload)
	mux.HandleFunc("GET /api/app-uninstall", s.handleAppUninstall)
	mux.HandleFunc("POST /api/app-uninstall", s.handleAppUninstall)
	mux.HandleFunc("GET /api/get-language", s.handleNotImplementedAPI("get-language"))
	mux.HandleFunc("POST /api/get-language", s.handleNotImplementedAPI("get-language"))
	mux.HandleFunc("GET /api/hello", s.handleHello)
	mux.HandleFunc("GET /api/translate-phrase", s.handleNotImplementedAPI("translate-phrase"))
	mux.HandleFunc("POST /api/translate-phrase", s.handleNotImplementedAPI("translate-phrase"))

	addr := ":" + port
	s.logger.Info("starting server", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		s.logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func (s *server) handleHome(w http.ResponseWriter, r *http.Request) {
	render(r.Context(), w, homePage())
}

func (s *server) handleAppDefinition(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"name":              "AI Content Importer",
		"documentationLink": "https://agilitycms.com/docs/apps",
		"description":       "Import content from DOCX and TXT files into Agility content items.",
		"version":           "0.1.0",
		"__sdkVersion":      "2.0.0",
		"configValues":      []any{},
		"capabilities": map[string]any{
			"contentItemSidebar": map[string]string{
				"description": "Import content from DOCX and TXT files.",
			},
			"installScreen": false,
		},
	})
}

func (s *server) handleInstall(w http.ResponseWriter, r *http.Request) {
	render(r.Context(), w, installPage())
}

func (s *server) handleHomeDashboard(w http.ResponseWriter, r *http.Request) {
	render(r.Context(), w, surfacePage("Home Dashboard", "AI Content Importer home dashboard route."))
}

func (s *server) handleContentDashboard(w http.ResponseWriter, r *http.Request) {
	render(r.Context(), w, surfacePage("Content Dashboard", "AI Content Importer content dashboard route."))
}

func (s *server) handlePagesDashboard(w http.ResponseWriter, r *http.Request) {
	render(r.Context(), w, surfacePage("Pages Dashboard", "AI Content Importer pages dashboard route."))
}

func (s *server) handleContentItemSidebar(w http.ResponseWriter, r *http.Request) {
	render(r.Context(), w, sidebarPage())
}

func (s *server) handleContentListSidebar(w http.ResponseWriter, r *http.Request) {
	render(r.Context(), w, surfacePage("Content List Sidebar", "AI Content Importer content list sidebar route."))
}

func (s *server) handlePageSidebar(w http.ResponseWriter, r *http.Request) {
	render(r.Context(), w, surfacePage("Page Sidebar", "AI Content Importer page sidebar route."))
}

func (s *server) handleUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)

	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		render(r.Context(), w, uploadResult(nil, "Upload failed. Keep files under 16 MB and try again."))
		return
	}

	files := r.MultipartForm.File["documents"]
	if len(files) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		render(r.Context(), w, uploadResult(nil, "Choose at least one .docx or .txt file."))
		return
	}

	results := make([]processedFile, 0, len(files))
	for _, file := range files {
		kind, err := acceptedDocumentType(file.Filename)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			render(r.Context(), w, uploadResult(nil, err.Error()))
			return
		}

		result := processedFile{
			uploadedFile: uploadedFile{
				Name: file.Filename,
				Size: file.Size,
				Type: kind,
			},
		}

		if kind != "DOCX" {
			result.Error = "TXT processing is not implemented yet."
			results = append(results, result)
			continue
		}

		updates, err := s.processDOCXUpload(r.Context(), file, r.FormValue("content_item_json"))
		if err != nil {
			result.Error = err.Error()
			s.logger.Error("docx import failed", "file", file.Filename, "error", err)
		} else {
			result.Updates = updates
		}

		results = append(results, result)
	}

	render(r.Context(), w, uploadResult(results, ""))
}

func (s *server) processDOCXUpload(ctx context.Context, header *multipart.FileHeader, contentItemJSON string) ([]fieldUpdate, error) {
	if strings.TrimSpace(contentItemJSON) == "" {
		contentItemJSON = "{}"
	}

	file, err := header.Open()
	if err != nil {
		return nil, fmt.Errorf("open upload: %w", err)
	}
	defer file.Close()

	docxXML, err := extractDOCXXML(file, header.Size)
	if err != nil {
		return nil, err
	}

	updates, err := newOllamaClient().mapDOCXToFields(ctx, contentItemJSON, docxXML)
	if err != nil {
		return nil, err
	}

	return updates, nil
}

func fieldUpdatesJSON(results []processedFile) string {
	updates := make([]fieldUpdate, 0)
	for _, result := range results {
		updates = append(updates, result.Updates...)
	}

	if len(updates) == 0 {
		return "[]"
	}

	data, err := json.Marshal(updates)
	if err != nil {
		slog.Error("field update json failed", "error", err)
		return "[]"
	}

	return string(data)
}

func successfulUpdateCount(results []processedFile) int {
	count := 0
	for _, result := range results {
		count += len(result.Updates)
	}
	return count
}

func (s *server) handleAppUninstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.logger.Info("app uninstall action fired")
	writeJSON(w, http.StatusOK, map[string]string{"status": "OK"})
}

func (s *server) handleHello(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"name": "AI Content Importer"})
}

func (s *server) handleNotImplementedAPI(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotImplemented, map[string]string{
			"status":  "not_implemented",
			"message": fmt.Sprintf("%s is not used by AI Content Importer yet.", name),
		})
	}
}

func acceptedDocumentType(name string) (string, error) {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".docx":
		return "DOCX", nil
	case ".txt":
		return "TXT", nil
	default:
		return "", fmt.Errorf("%s is not supported. Upload .docx or .txt files only.", name)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("json response failed", "error", err)
	}
}

func render(ctx context.Context, w http.ResponseWriter, c interface {
	Render(context.Context, io.Writer) error
}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := c.Render(ctx, w); err != nil {
		if !errors.Is(err, context.Canceled) {
			slog.Error("render failed", "error", err)
		}
	}
}

func env(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
