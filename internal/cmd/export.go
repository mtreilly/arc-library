// Copyright (c) 2025 Arc Engineering
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mtreilly/arc-library/internal/library"
	"github.com/yourorg/arc-sdk/config"
)

func newExportCmd(cfg *config.Config, store library.LibraryStore) *cobra.Command {
	var (
		format   string // "bibtex", "markdown", "json"
		output   string // file path or "-" for stdout
		tag      string
		source   string
		docType  string
		collections []string
	)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export library documents to various formats",
		Long:  "Export your library to formats like BibTeX, Markdown, or JSON for use in other tools.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get documents (apply filters)
			docs, err := store.ListDocuments(&library.ListOptions{
				Tag:    tag,
				Source: source,
				Type:   docType,
			})
			if err != nil {
				return fmt.Errorf("list documents: %w", err)
			}

			// Filter by collections if specified
			if len(collections) > 0 {
				var filtered []*library.Document
				for _, doc := range docs {
					for _, collName := range collections {
						c, _ := store.GetCollection(collName)
						if c != nil {
							for _, did := range c.DocumentIDs {
								if did == doc.ID {
									filtered = append(filtered, doc)
									break
								}
							}
						}
					}
				}
				docs = filtered
			}

			var outBytes []byte

			switch format {
			case "bibtex":
				outBytes, err = exportBibTeX(docs)
			case "markdown":
				outBytes, err = exportMarkdown(docs, store)
			case "json":
				outBytes, err = exportJSON(docs)
			case "ris":
				outBytes, err = exportRIS(docs)
			default:
				return fmt.Errorf("unsupported format: %s (choose bibtex, markdown, json, ris)", format)
			}
			if err != nil {
				return fmt.Errorf("export %s: %w", format, err)
			}

			if output == "-" || output == "" {
				fmt.Println(string(outBytes))
			} else {
				// Write to file (TODO)
				return fmt.Errorf("file output not yet implemented")
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "bibtex", "Export format: bibtex, markdown, json")
	cmd.Flags().StringVarP(&output, "output", "o", "-", "Output file (default: stdout)")
	cmd.Flags().StringVarP(&tag, "tag", "t", "", "Filter by tag")
	cmd.Flags().StringVarP(&source, "source", "s", "", "Filter by source")
	cmd.Flags().StringVarP(&docType, "type", "", "", "Filter by document type")
	cmd.Flags().StringSliceVarP(&collections, "collection", "c", nil, "Filter by collection name (can be repeated)")

	return cmd
}

// exportBibTeX converts documents to BibTeX format.
func exportBibTeX(docs []*library.Document) ([]byte, error) {
	var buf bytes.Buffer

	for _, doc := range docs {
		// Generate a BibTeX entry type and key
		entryType := "article" // default
		if doc.Type == library.DocTypeBook {
			entryType = "book"
		} else if doc.Type == library.DocTypeOther {
			entryType = "misc"
		}

		// Generate citation key from authors + year or source_id
		key := "unknown"
		if len(doc.Authors) > 0 {
			author := doc.Authors[0]
			parts := strings.Fields(author)
			if len(parts) > 0 {
				key = strings.ToLower(parts[0])
			}
		}
		if doc.Source == "arxiv" && doc.SourceID != "" {
			key = doc.SourceID
		} else if doc.Source == "doi" && doc.SourceID != "" {
			key = strings.ReplaceAll(doc.SourceID, "/", "_")
		}
		// Add year if available
		if year, ok := doc.Meta["year"].(int); ok {
			key = fmt.Sprintf("%s%d", key, year)
		}

		buf.WriteString(fmt.Sprintf("@%s{%s,\n", entryType, key))

		// Title
		if doc.Title != "" {
			buf.WriteString(fmt.Sprintf("  title = {%s},\n", escapeBibTeX(doc.Title)))
		}

		// Authors
		if len(doc.Authors) > 0 {
			buf.WriteString(fmt.Sprintf("  author = {%s},\n", strings.Join(doc.Authors, " and ")))
		}

		// Abstract
		if doc.Abstract != "" {
			buf.WriteString(fmt.Sprintf("  abstract = {%s},\n", escapeBibTeX(doc.Abstract)))
		}

		// Year from Meta or timestamps
		if year, ok := doc.Meta["year"].(int); ok {
			buf.WriteString(fmt.Sprintf("  year = {%d},\n", year))
		} else {
			buf.WriteString(fmt.Sprintf("  year = {%d},\n", doc.CreatedAt.Year()))
		}

		// Journal / container
		if journal, ok := doc.Meta["journal"].(string); ok {
			buf.WriteString(fmt.Sprintf("  journal = {%s},\n", journal))
		}

		// URL
		if url, ok := doc.Meta["url"].(string); ok {
			buf.WriteString(fmt.Sprintf("  url = {%s},\n", url))
		}

		// arXiv ID
		if doc.Source == "arxiv" && doc.SourceID != "" {
			buf.WriteString(fmt.Sprintf("  eprint = {%s},\n", doc.SourceID))
			buf.WriteString("  archivePrefix = {arXiv},\n")
		}

		// DOI
		if doc.Source == "doi" && doc.SourceID != "" {
			buf.WriteString(fmt.Sprintf("  doi = {%s},\n", doc.SourceID))
		}

		// Path (local file) - custom field
		if doc.Path != "" {
			buf.WriteString(fmt.Sprintf("  file = {%s},\n", doc.Path))
		}

		// Tags as keywords
		if len(doc.Tags) > 0 {
			buf.WriteString(fmt.Sprintf("  keywords = {%s}},\n", strings.Join(doc.Tags, ", ")))
		} else {
			// Remove trailing comma from previous line if no keywords
			buf.Truncate(buf.Len() - 2) // remove ",\n"
			buf.WriteString("\n")
		}

		buf.WriteString("\n")
	}

	return buf.Bytes(), nil
}

// escapeBibTeX escapes special characters for BibTeX.
func escapeBibTeX(s string) string {
	// Basic escaping: curly braces, quotes, backslashes, commas
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "{", "\\{")
	s = strings.ReplaceAll(s, "}", "\\}")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}

// exportMarkdown converts documents to a Markdown notes collection.
func exportMarkdown(docs []*library.Document, store library.LibraryStore) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString("# Library Export\n\n")
	buf.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().Format(time.RFC3339)))
	buf.WriteString(fmt.Sprintf("Total documents: %d\n\n---\n\n", len(docs)))

	for _, doc := range docs {
		buf.WriteString(fmt.Sprintf("## %s\n\n", doc.Title))

		// Metadata
		buf.WriteString("**Type:** " + string(doc.Type) + "\n\n")
		if len(doc.Authors) > 0 {
			buf.WriteString("**Authors:** " + strings.Join(doc.Authors, ", ") + "\n\n")
		}
		if doc.Source != "" {
			buf.WriteString(fmt.Sprintf("**Source:** %s %s\n\n", doc.Source, doc.SourceID))
		}
		if doc.Abstract != "" {
			buf.WriteString("**Abstract**\n\n")
			buf.WriteString(doc.Abstract + "\n\n")
		}
		if doc.FullText != "" {
			buf.WriteString("**Full Text**\n\n")
			buf.WriteString(doc.FullText[:min(2000, len(doc.FullText))] + "...\n\n")
		}
		if len(doc.Tags) > 0 {
			buf.WriteString("**Tags:** " + strings.Join(doc.Tags, ", ") + "\n\n")
		}
		if doc.Notes != "" {
			buf.WriteString("**Notes**\n\n")
			buf.WriteString(doc.Notes + "\n\n")
		}
		if doc.Rating > 0 {
			buf.WriteString(fmt.Sprintf("**Rating:** %d/5\n\n", doc.Rating))
		}

		// Annotations for this document
		anns, _ := store.GetAnnotations(doc.ID)
		if len(anns) > 0 {
			buf.WriteString("### Annotations\n\n")
			for _, a := range anns {
				buf.WriteString(fmt.Sprintf("- [%s] %s\n", a.Type, a.Content))
				if a.Page > 0 {
					buf.WriteString(fmt.Sprintf("  (page %d)\n", a.Page))
				}
				buf.WriteString("\n")
			}
		}

		buf.WriteString("---\n\n")
	}

	return buf.Bytes(), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// exportJSON just re-serializes documents (could add filtering/transformation)
func exportJSON(docs []*library.Document) ([]byte, error) {
	return json.MarshalIndent(docs, "", "  ")
}

// exportRIS converts documents to RIS format (reference standard).
func exportRIS(docs []*library.Document) ([]byte, error) {
	var buf bytes.Buffer

	for _, doc := range docs {
		// RIS type
		risType := "JOUR" // default journal article
		if doc.Type == library.DocTypeBook {
			risType = "BOOK"
		} else if doc.Type == library.DocTypeOther {
			risType = "GEN"
		}
		buf.WriteString(fmt.Sprintf("TY  - %s\n", risType))

		// Title
		if doc.Title != "" {
			buf.WriteString(fmt.Sprintf("TI  - %s\n", doc.Title))
		}

		// Authors
		for _, author := range doc.Authors {
			buf.WriteString(fmt.Sprintf("AU  - %s\n", author))
		}

		// Year
		year := doc.CreatedAt.Year()
		if y, ok := doc.Meta["year"].(int); ok {
			year = y
		}
		if year > 0 {
			buf.WriteString(fmt.Sprintf("PY  - %d\n", year))
		}

		// Journal / container
		if journal, ok := doc.Meta["journal"].(string); ok && journal != "" {
			buf.WriteString(fmt.Sprintf("JO  - %s\n", journal))
		}

		// Abstract
		if doc.Abstract != "" {
			buf.WriteString(fmt.Sprintf("AB  - %s\n", doc.Abstract))
		}

		// DOI
		if doc.Source == "doi" && doc.SourceID != "" {
			buf.WriteString(fmt.Sprintf("DO  - %s\n", doc.SourceID))
		}

		// arXiv ID
		if doc.Source == "arxiv" && doc.SourceID != "" {
			buf.WriteString(fmt.Sprintf("UR  - https://arxiv.org/abs/%s\n", doc.SourceID))
		}

		// URL if present in meta
		if url, ok := doc.Meta["url"].(string); ok && url != "" {
			buf.WriteString(fmt.Sprintf("UR  - %s\n", url))
		}

		// Local file
		if doc.Path != "" {
			buf.WriteString(fmt.Sprintf("L1  - %s\n", doc.Path))
		}

		// Tags as keywords
		if len(doc.Tags) > 0 {
			for _, tag := range doc.Tags {
				buf.WriteString(fmt.Sprintf("KW  - %s\n", tag))
			}
		}

		// End record
		buf.WriteString("ER  - \n\n")
	}

	return buf.Bytes(), nil
}
