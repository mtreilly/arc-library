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

func newImportCmd(cfg *config.Config, store *library.Store) *cobra.Command {
	var tags []string
	var collection string

	cmd := &cobra.Command{
		Use:   "import <path>",
		Short: "Import papers into the library",
		Long: `Import papers from the filesystem into the library database.

Examples:
  arc-library import ~/papers/2304.00067           # Import single paper
  arc-library import ~/papers                      # Import all papers in directory
  arc-library import ~/papers --tag ml --tag nlp   # Import with tags
  arc-library import ~/papers -c "my-project"      # Add to collection`,
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

			var paths []string
			if info.IsDir() {
				// Look for paper directories (those with meta.yaml)
				entries, err := os.ReadDir(importPath)
				if err != nil {
					return err
				}
				for _, entry := range entries {
					if entry.IsDir() {
						metaPath := filepath.Join(importPath, entry.Name(), "meta.yaml")
						if _, err := os.Stat(metaPath); err == nil {
							paths = append(paths, filepath.Join(importPath, entry.Name()))
						}
					}
				}
				// Also check if the path itself is a paper directory
				metaPath := filepath.Join(importPath, "meta.yaml")
				if _, err := os.Stat(metaPath); err == nil {
					paths = []string{importPath}
				}
			} else {
				return fmt.Errorf("expected directory path, got file")
			}

			if len(paths) == 0 {
				return fmt.Errorf("no papers found at %s", importPath)
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

			for _, paperPath := range paths {
				// Check if already imported
				existing, _ := store.GetPaperByPath(paperPath)
				if existing != nil {
					skipped++
					continue
				}

				// Read meta.yaml
				metaPath := filepath.Join(paperPath, "meta.yaml")
				meta, err := readArxivMeta(metaPath)
				if err != nil {
					fmt.Printf("  Warning: could not read %s: %v\n", metaPath, err)
					continue
				}

				paper := &library.Paper{
					Path:     paperPath,
					Source:   meta.SourceType,
					SourceID: meta.ArxivID,
					Title:    meta.Title,
					Authors:  extractAuthorNames(meta.Authors),
					Abstract: meta.Abstract,
					Tags:     tags,
				}

				if err := store.AddPaper(paper); err != nil {
					fmt.Printf("  Warning: could not import %s: %v\n", paperPath, err)
					continue
				}

				// Add to collection if specified
				if collectionID != "" {
					store.AddToCollection(collectionID, paper.ID)
				}

				fmt.Printf("Imported: %s - %s\n", meta.ArxivID, truncate(meta.Title, 50))
				imported++
			}

			fmt.Printf("\nImported %d paper(s), skipped %d already in library.\n", imported, skipped)
			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&tags, "tag", "t", nil, "Tags to apply to imported papers")
	cmd.Flags().StringVarP(&collection, "collection", "c", "", "Add papers to collection")

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

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
