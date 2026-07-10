package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"sort"
	"strings"
)

const maxExtractedDocxXMLBytes = 4 << 20

func extractDOCXXML(file multipart.File, size int64) (string, error) {
	if seeker, ok := file.(io.Seeker); ok {
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			return "", fmt.Errorf("reset docx stream: %w", err)
		}
	}

	data, err := io.ReadAll(io.LimitReader(file, maxUploadBytes+1))
	if err != nil {
		return "", fmt.Errorf("read docx: %w", err)
	}
	if int64(len(data)) > maxUploadBytes || size > maxUploadBytes {
		return "", fmt.Errorf("DOCX is too large")
	}

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("open docx zip: %w", err)
	}

	files := make([]*zip.File, 0, len(reader.File))
	for _, f := range reader.File {
		if shouldExtractDocxPart(f.Name) {
			files = append(files, f)
		}
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name < files[j].Name
	})

	var out strings.Builder
	for _, f := range files {
		part, err := readZipPart(f, maxExtractedDocxXMLBytes-out.Len())
		if err != nil {
			return "", err
		}
		if out.Len()+len(part) > maxExtractedDocxXMLBytes {
			return "", fmt.Errorf("extracted DOCX XML is too large")
		}
		out.WriteString("\n<!-- ")
		out.WriteString(f.Name)
		out.WriteString(" -->\n")
		out.Write(part)
		out.WriteByte('\n')
	}

	xml := strings.TrimSpace(out.String())
	if xml == "" {
		return "", fmt.Errorf("DOCX did not contain extractable XML")
	}
	return xml, nil
}

func shouldExtractDocxPart(name string) bool {
	normalized := filepath.ToSlash(name)
	if strings.HasPrefix(normalized, "word/media/") {
		return false
	}
	if strings.HasSuffix(normalized, ".xml") || strings.HasSuffix(normalized, ".rels") {
		return true
	}
	return false
}

func readZipPart(f *zip.File, remaining int) ([]byte, error) {
	if remaining <= 0 {
		return nil, fmt.Errorf("extracted DOCX XML is too large")
	}

	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("open docx part %s: %w", f.Name, err)
	}
	defer rc.Close()

	data, err := io.ReadAll(io.LimitReader(rc, int64(remaining)+1))
	if err != nil {
		return nil, fmt.Errorf("read docx part %s: %w", f.Name, err)
	}
	if len(data) > remaining {
		return nil, fmt.Errorf("extracted DOCX XML is too large")
	}
	return data, nil
}
