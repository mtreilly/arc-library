// Copyright (c) 2025 Arc Engineering
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mtreilly/arc-library/internal/library"
	"github.com/yourorg/arc-sdk/config"
	"gopkg.in/yaml.v3"
)

func newImportCmd(cfg *config.Config, store library.LibraryStore) *cobra.Command {
	var tags []string
	var collection string

	// PDF import flags
	var (
		extractText bool
		resolveDOI  bool
		doiFlag     string
		docType     string
		sourceFlag  string
		titleFlag   string
		authorsFlag string
		abstractFlag string
	)

	cmd := &cobra.Command{
		Use:   "import <path>",
		Short: "Import documents into the library",
		Long: `Import documents from the filesystem into the library database.

Supported sources:
- Directory with meta.yaml (as created by arc-arxiv)
- PDF file(s) with optional metadata flags

Examples:
  arc-library import ~/papers/2304.00067                    # Import meta directory
  arc-library import ~/papers/paper.pdf --title "My Paper" # Import single PDF
  arc-library import ~/papers --tag ml --collection proj    # Import all meta dirs with tags
  arc-library import ~/papers --recursive --extract-text   # Import all PDFs with full text`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			importPath := args[0]

			// Expand ~ to home directory
			if strings.HasPrefix(importPath, "~") {
				home, _ := os.UserHomeDir()
				importPath = filepath.Join(home, importPath[1:])
			}

			info, err := os.Stat(importPath)
			if err != nil {
				return fmt.Errorf("path not found: %s", importPath)
			}

			// Determine import mode
			var pathsToImport []string
			var isPDFImport bool

			if info.IsDir() {
				// Check if this looks like a meta directory (has meta.yaml)
				metaPath := filepath.Join(importPath, "meta.yaml")
				if _, err := os.Stat(metaPath); err == nil {
					// Single document directory
					pathsToImport = []string{importPath}
				} else {
					// Scan directory for PDF files (non-recursive)
					entries, err := os.ReadDir(importPath)
					if err != nil {
						return err
					}
					for _, e := range entries {
						if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".pdf") {
							pathsToImport = append(pathsToImport, filepath.Join(importPath, e.Name()))
						}
					}
					if len(pathsToImport) == 0 {
						return fmt.Errorf("no meta.yaml or PDF files found in %s", importPath)
					}
					isPDFImport = true
				}
			} else if strings.EqualFold(filepath.Ext(importPath), ".pdf") {
				// Single PDF file
				pathsToImport = []string{importPath}
				isPDFImport = true
			} else {
				return fmt.Errorf("unsupported file type: %s (expected directory or .pdf)", importPath)
			}

			// Get or create collection if specified
			var collectionID string
			if collection != "" {
				c, err := store.GetCollection(collection)
				if err != nil {
					return err
				}
				if c == nil {
					c, err = store.CreateCollection(collection, "")
					if err != nil {
						return fmt.Errorf("create collection: %w", err)
					}
					fmt.Printf("Created collection: %s\n", collection)
				}
				collectionID = c.ID
			}

			imported := 0
			skipped := 0

			for _, path := range pathsToImport {
				// Check if already imported
				existing, _ := store.GetDocumentByPath(path)
				if existing != nil {
					skipped++
					continue
				}

				var doc *library.Document

				if isPDFImport {
					// PDF import
					title := titleFlag
					if title == "" {
						// Use filename as title
						title = filepath.Base(path)
						title = strings.TrimSuffix(title, filepath.Ext(title))
					}

					authors := strings.Split(authorsFlag, ",")
					for i, a := range authors {
						authors[i] = strings.TrimSpace(a)
					}
					if len(authors) == 0 {
						authors = []string{}
					}

					doc = &library.Document{
						Path:   path,
						Source: sourceFlag,
						Title:  title,
						Authors: authors,
						Abstract: abstractFlag,
						Tags:   tags,
						Type:   library.DocTypePaper, // default
					}

					// If extractText flag, try to extract full text
					if extractText {
						fmt.Printf("  Extracting text from %s...\n", filepath.Base(path))
						text, err := library.PDFTextExtractor(path)
						if err != nil {
							fmt.Printf("    Warning: text extraction failed: %v\n", err)
						} else {
							doc.FullText = text
						}
					}

					// If DOI provided, resolve metadata
					if doiFlag != "" {
						doc.Source = "doi"
						doc.SourceID = strings.TrimPrefix(doiFlag, "doi:")
						if resolveDOI {
							fmt.Printf("  Resolving DOI %s...\n", doc.SourceID)
							meta, err := library.DOIResolver(doc.SourceID)
							if err != nil {
								fmt.Printf("    Warning: DOI resolution failed: %v\n", err)
							} else {
								// Override/merge metadata from DOI
								if doc.Title == "" {
									if t, ok := meta["title"].(string); ok {
										doc.Title = t
									}
								}
								if len(doc.Authors) == 0 {
									if a, ok := meta["authors"].([]string); ok {
										doc.Authors = a
									}
								}
								if doc.Abstract == "" {
									if a, ok := meta["abstract"].(string); ok {
										doc.Abstract = a
									}
								}
								// Could also set year, journal etc. in Meta
								doc.Meta = meta
							}
						}
					}

				} else {
					// Meta directory import (existing behavior)
					metaPath := filepath.Join(path, "meta.yaml")
					meta, err := readArxivMeta(metaPath)
					if err != nil {
						fmt.Printf("  Warning: could not read %s: %v\n", metaPath, err)
						continue
					}

					doc = &library.Document{
						Path:     path,
						Source:   meta.SourceType,
						SourceID: meta.ArxivID,
						Title:    meta.Title,
						Authors:  extractAuthorNames(meta.Authors),
						Abstract: meta.Abstract,
						Tags:     tags,
					}
				}

				// Set type if specified
				if docType != "" {
					doc.Type = library.DocumentType(docType)
				} else if doc.Type == "" {
					doc.Type = library.DocTypePaper
				}

				if err := store.AddDocument(doc); err != nil {
					fmt.Printf("  Warning: could not import %s: %v\n", path, err)
					continue
				}

				// Add to collection if specified
				if collectionID != "" {
					store.AddToCollection(collectionID, doc.ID)
				}

				fmt.Printf("Imported: %s - %s\n", doc.SourceID, truncate(doc.Title, 50))
				imported++
			}

			fmt.Printf("\nImported %d document(s), skipped %d already in library.\n", imported, skipped)
			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&tags, "tag", "t", nil, "Tags to apply to imported documents")
	cmd.Flags().StringVarP(&collection, "collection", "c", "", "Add documents to collection")

	// PDF import specific flags
	cmd.Flags().BoolVarP(&extractText, "extract-text", "e", false, "Extract full text from PDFs (requires pdftotext)")
	cmd.Flags().BoolVarP(&resolveDOI, "resolve-doi", "r", false, "Resolve DOI metadata (Crossref)")
	cmd.Flags().StringVar(&doiFlag, "doi", "", "DOI to assign to the document (e.g., 10.1234/5678)")
	cmd.Flags().StringVar(&docType, "type", "", "Document type (paper, book, article, video, note, repo, other)")
	cmd.Flags().StringVar(&sourceFlag, "source", "", "Source identifier (e.g., local, arxiv, url)")
	cmd.Flags().StringVar(&titleFlag, "title", "", "Title for PDF import (default: filename)")
	cmd.Flags().StringVar(&authorsFlag, "authors", "", "Comma-separated list of authors")
	cmd.Flags().StringVar(&abstractFlag, "abstract", "", "Abstract or summary")

	return cmd
}

// arxivMeta matches the structure from arc-arxiv
type arxivMeta struct {
	ID         string       `yaml:"id"`
	ArxivID    string       `yaml:"arxiv_id"`
	Title      string       `yaml:"title"`
	SourceType string       `yaml:"source_type"`
	Authors    []arxivAuthor `yaml:"authors"`
	Abstract   string       `yaml:"abstract"`
}

type arxivAuthor struct {
	Name        string `yaml:"name"`
	Affiliation string `yaml:"affiliation,omitempty"`
}

func readArxivMeta(path string) (*arxivMeta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var meta arxivMeta
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func extractAuthorNames(authors []arxivAuthor) []string {
	names := make([]string, 0, len(authors))
	for _, a := range authors {
		names = append(names, a.Name)
	}
	return names
}
