// Copyright (c) 2025 Arc Engineering
// SPDX-License-Identifier: MIT

package library

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// DOIResolver resolves a DOI to document metadata using Crossref API.
// It returns a map of metadata fields (title, authors, etc.)
func DOIResolver(doi string) (JSONMap, error) {
	// Normalize DOI: ensure it starts with https://doi.org/
	if !strings.HasPrefix(doi, "http") {
		doi = "https://doi.org/" + strings.TrimPrefix(doi, "doi:")
	}

	// Use Crossref API: https://api.crossref.org/works/<doi>
	url := "https://api.crossref.org/works/" + strings.TrimPrefix(doi, "https://doi.org/")
	// It's okay to not set User-Agent; but Crossref requires a User-Agent. Let's set a generic one.
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "arc-library/1.0 (mailto:you@example.com)")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("query doi: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("DOI lookup failed: %s: %s", resp.Status, string(body))
	}

	var envelope struct {
		Message struct {
			Title  []string          `json:"title"`
			Author []struct {
				Given  string `json:"given"`
				Family string `json:"family"`
			} `json:"author"`
			Abstract string `json:"abstract"`
			URL       string `json:"URL"`
			ISBN      []string `json:"ISBN"`
			Issue     string `json:"issue"`
			Volume    string `json:"volume"`
			Page      string `json:"page"`
			Published struct {
				DateParts [][]int `json:"date-parts"`
			} `json:"published"`
			JournalTitle string `json:"container-title"`
		} `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	meta := make(JSONMap)

	// Title
	if len(envelope.Message.Title) > 0 {
		meta["title"] = envelope.Message.Title[0]
	}

	// Authors
	authors := make([]string, 0, len(envelope.Message.Author))
	for _, a := range envelope.Message.Author {
		name := a.Family
		if a.Given != "" {
			name = a.Given + " " + name
		}
		authors = append(authors, name)
	}
	meta["authors"] = authors

	// Abstract
	if envelope.Message.Abstract != "" {
		meta["abstract"] = strings.ReplaceAll(envelope.Message.Abstract, "<jats:p>", "")
		meta["abstract"] = strings.ReplaceAll(meta["abstract"].(string), "</jats:p>", "")
	}

	// URL
	if envelope.Message.URL != "" {
		meta["url"] = envelope.Message.URL
	}

	// Publication date
	if len(envelope.Message.Published.DateParts) > 0 {
		year := envelope.Message.Published.DateParts[0][0]
		meta["year"] = year
	}

	// Journal / container title
	if envelope.Message.JournalTitle != "" {
		meta["journal"] = envelope.Message.JournalTitle
	}

	return meta, nil
}

// PDFTextExtractor extracts text from a PDF file using external tool (pdftotext).
// It returns the full text content.
// If pdftotext is not available, it returns an error.
func PDFTextExtractor(pdfPath string) (string, error) {
	// Try pdftotext (from poppler)
	cmd := exec.Command("pdftotext", pdfPath, "-")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = bytes.NewBuffer(nil)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pdftotext failed: %w (is poppler installed?)", err)
	}
	text := out.String()
	// Clean up excessive whitespace
	text = strings.TrimSpace(text)
	return text, nil
}

// Suggest: In the future, you could also use a pure Go PDF parser like "unidoc/unipdf"
// but that requires a commercial license for full features. For now, rely on pdftotext.
