package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
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

func main() {
	port := env("PORT", "8080")

	s := &server{
		logger: slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.HandleFunc("GET /", s.handleHome)
	mux.HandleFunc("GET /install", s.handleInstall)
	mux.HandleFunc("GET /content-item-sidebar", s.handleContentItemSidebar)
	mux.HandleFunc("POST /content-item-sidebar/upload", s.handleUpload)

	addr := ":" + port
	s.logger.Info("starting server", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		s.logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func (s *server) handleHome(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/content-item-sidebar", http.StatusFound)
}

func (s *server) handleInstall(w http.ResponseWriter, r *http.Request) {
	render(r.Context(), w, installPage())
}

func (s *server) handleContentItemSidebar(w http.ResponseWriter, r *http.Request) {
	render(r.Context(), w, sidebarPage())
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

	uploaded := make([]uploadedFile, 0, len(files))
	for _, file := range files {
		kind, err := acceptedDocumentType(file.Filename)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			render(r.Context(), w, uploadResult(nil, err.Error()))
			return
		}

		uploaded = append(uploaded, uploadedFile{
			Name: file.Filename,
			Size: file.Size,
			Type: kind,
		})
	}

	render(r.Context(), w, uploadResult(uploaded, ""))
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
