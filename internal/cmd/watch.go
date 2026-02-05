// Copyright (c) 2025 Arc Engineering
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mtreilly/arc-library/internal/library"
	"github.com/spf13/cobra"
	"github.com/yourorg/arc-sdk/config"
)

func newWatchCmd(cfg *config.Config, store library.LibraryStore) *cobra.Command {
	var (
		dir           string
		recursive     bool
		extractText   bool
		resolveDOI    bool
		tags          []string
		collection    string
		debounceMs    int
		oneShot       bool
	)

	cmd := &cobra.Command{
		Use:   "watch [directory]",
		Short: "Watch a folder for new PDFs and auto-import",
		Long: `Monitor a directory for new PDF files and automatically import them into the library.

Examples:
  arc-library watch ~/Downloads/papers
  arc-library watch ~/Dropbox --recursive --extract-text --tag "inbox"
  arc-library watch ~/Papers --collection "To Read" --one-shot`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Determine watch directory
			if len(args) > 0 {
				dir = args[0]
			}
			if dir == "" {
				home, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("cannot determine home directory: %w", err)
				}
				dir = filepath.Join(home, "Downloads")
			}

			// Verify directory exists
			info, err := os.Stat(dir)
			if err != nil {
				return fmt.Errorf("cannot access directory %s: %w", dir, err)
			}
			if !info.IsDir() {
				return fmt.Errorf("%s is not a directory", dir)
			}

			// One-shot: just process existing files
			if oneShot {
				return processExistingFiles(dir, recursive, store, extractText, resolveDOI, tags, collection)
			}

			// Start watching
			return watchDirectory(dir, recursive, store, extractText, resolveDOI, tags, collection, debounceMs)
		},
	}

	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Watch subdirectories recursively")
cmd.Flags().BoolVar(&extractText, "extract-text", false, "Extract full text from PDFs")
	cmd.Flags().BoolVar(&resolveDOI, "resolve-doi", false, "Try to resolve DOI from PDF metadata")
	cmd.Flags().StringSliceVarP(&tags, "tag", "t", nil, "Tags to apply to imported documents")
	cmd.Flags().StringVarP(&collection, "collection", "c", "", "Add imported documents to collection")
	cmd.Flags().IntVar(&debounceMs, "debounce", 1000, "Debounce milliseconds for file events")
	cmd.Flags().BoolVar(&oneShot, "one-shot", false, "Process existing files and exit (don't watch)")

	return cmd
}

func watchDirectory(dir string, recursive bool, store library.LibraryStore, extractText, resolveDOI bool, tags []string, collection string, debounceMs int) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	defer watcher.Close()

	// Track pending imports with debounce
	pending := make(map[string]*time.Timer)
	var pendingMu sync.Mutex

	// Add directories to watch
	if recursive {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				if err := watcher.Add(path); err != nil {
					log.Printf("Warning: cannot watch %s: %v", path, err)
				} else {
					log.Printf("Watching: %s", path)
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("walk directories: %w", err)
		}
	} else {
		if err := watcher.Add(dir); err != nil {
			return fmt.Errorf("watch directory: %w", err)
		}
		log.Printf("Watching: %s", dir)
	}

	log.Println("Press Ctrl+C to stop watching")

	// Process events
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			// Only care about PDFs
			if !strings.HasSuffix(strings.ToLower(event.Name), ".pdf") {
				continue
			}

			// Only process on create or rename (new files)
			if event.Op&(fsnotify.Create|fsnotify.Rename) == 0 {
				continue
			}

			// Debounce: reset timer if file is still being written
			pendingMu.Lock()
			if timer, exists := pending[event.Name]; exists {
				timer.Stop()
			}
			pending[event.Name] = time.AfterFunc(time.Duration(debounceMs)*time.Millisecond, func() {
				pendingMu.Lock()
				delete(pending, event.Name)
				pendingMu.Unlock()

				if err := importFile(event.Name, store, extractText, resolveDOI, tags, collection); err != nil {
					log.Printf("Failed to import %s: %v", event.Name, err)
				}
			})
			pendingMu.Unlock()

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			log.Printf("Watcher error: %v", err)
		}
	}
}

func processExistingFiles(dir string, recursive bool, store library.LibraryStore, extractText, resolveDOI bool, tags []string, collection string) error {
	var files []string

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".pdf") {
			files = append(files, path)
		}
		if info.IsDir() && !recursive && path != dir {
			return filepath.SkipDir
		}
		return nil
	}

	if err := filepath.Walk(dir, walkFn); err != nil {
		return fmt.Errorf("walk directory: %w", err)
	}

	if len(files) == 0 {
		fmt.Println("No PDF files found")
		return nil
	}

	fmt.Printf("Found %d PDF file(s), importing...\n", len(files))

	imported := 0
	failed := 0
	for _, f := range files {
		if err := importFile(f, store, extractText, resolveDOI, tags, collection); err != nil {
			log.Printf("Failed: %s - %v", f, err)
			failed++
		} else {
			imported++
		}
	}

	fmt.Printf("\nImported: %d, Failed: %d\n", imported, failed)
	return nil
}

func importFile(path string, store library.LibraryStore, extractText, resolveDOI bool, tags []string, collection string) error {
	log.Printf("Importing: %s", path)

	doc := &library.Document{
		Path:      path,
		Source:    "local",
		Type:      library.DocTypePaper, // default
		Title:     filepath.Base(path),
		Tags:      tags,
		Status:    library.StatusUnread,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Try to extract text if requested
	if extractText {
		text, err := library.PDFTextExtractor(path)
		if err == nil && text != "" {
			doc.FullText = text
			// Try to extract title from first line
			lines := strings.Split(text, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if len(line) > 10 && len(line) < 200 {
					doc.Title = line
					break
				}
			}
		}
	}

	// Try to resolve DOI if requested
	if resolveDOI {
		// Try to find DOI in text or filename
		doi := ""
		if doc.FullText != "" {
			// Simple DOI regex extraction
			if idx := strings.Index(doc.FullText, "10."); idx != -1 {
				// Extract potential DOI (simplified)
				rest := doc.FullText[idx:]
				if space := strings.IndexAny(rest, " \n\t"); space != -1 {
					doi = rest[:space]
				}
			}
		}
		if doi == "" {
			// Try from filename - common pattern: 10.xxxx in filename
			base := filepath.Base(path)
			if idx := strings.Index(base, "10."); idx != -1 {
				doi = base[idx:]
				if ext := strings.LastIndex(doi, "."); ext != -1 {
					doi = doi[:ext]
				}
			}
		}
		if doi != "" {
			meta, err := library.DOIResolver(doi)
			if err == nil {
				if title, ok := meta["title"].(string); ok {
					doc.Title = title
				}
				if authors, ok := meta["authors"].([]string); ok {
					doc.Authors = authors
				}
				if abstract, ok := meta["abstract"].(string); ok {
					doc.Abstract = abstract
				}
				doc.Source = "doi"
				doc.SourceID = doi
				if doc.Meta == nil {
					doc.Meta = make(map[string]any)
				}
				doc.Meta["doi"] = doi
				if year, ok := meta["year"].(int); ok {
					doc.Meta["year"] = year
				}
				if journal, ok := meta["journal"].(string); ok {
					doc.Meta["journal"] = journal
				}
			}
		}
	}

	if err := store.AddDocument(doc); err != nil {
		return fmt.Errorf("add document: %w", err)
	}

	// Add to collection if specified
	if collection != "" {
		coll, err := store.GetCollection(collection)
		if err != nil {
			return fmt.Errorf("get collection: %w", err)
		}
		if coll == nil {
			// Create collection if it doesn't exist
			coll, err = store.CreateCollection(collection, "Auto-created by watch")
			if err != nil {
				return fmt.Errorf("create collection: %w", err)
			}
		}
		if err := store.AddToCollection(coll.ID, doc.ID); err != nil {
			return fmt.Errorf("add to collection: %w", err)
		}
	}

	log.Printf("Imported: %s (ID: %s)", doc.Title, doc.ID)
	return nil
}
