package cmd

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/mtreilly/arc-library/internal/library"
	"github.com/spf13/cobra"
	"github.com/yourorg/arc-sdk/config"
)

func newDuplicatesCmd(cfg *config.Config, store library.LibraryStore) *cobra.Command {
	var (
		threshold float64 // similarity threshold (0-1)
	)

	cmd := &cobra.Command{
		Use:   "duplicates",
		Short: "Detect duplicate or similar documents",
		Long:  "Scan your library for potential duplicates by comparing titles and metadata.",
		RunE: func(cmd *cobra.Command, args []string) error {
			docs, err := store.ListDocuments(&library.ListOptions{})
			if err != nil {
				return fmt.Errorf("list documents: %w", err)
			}

			if len(docs) < 2 {
				fmt.Println("Not enough documents to compare.")
				return nil
			}

			var duplicates []duplicatePair

			// Compare each pair (O(n^2) but fine for moderate sized libraries)
			for i := 0; i < len(docs); i++ {
				for j := i + 1; j < len(docs); j++ {
					d1, d2 := docs[i], docs[j]

					// Skip identical documents (same ID)
					if d1.ID == d2.ID {
						continue
					}

					// Check DOI/source_id first (exact match is strong signal)
					if (d1.Source == d2.Source && d1.SourceID != "" && d1.SourceID == d2.SourceID) ||
						(metaDoi(d1) == metaDoi(d2) && metaDoi(d1) != "") {
						duplicates = append(duplicates, duplicatePair{
							Doc1:     d1,
							Doc2:     d2,
							Score:    1.0,
							Reason:   "matching " + d1.Source + " ID",
						})
						continue
					}

					// Title similarity
					sim := titleSimilarity(d1.Title, d2.Title)
					if sim >= threshold {
						reason := fmt.Sprintf("title similarity %.2f", sim)
						duplicates = append(duplicates, duplicatePair{
							Doc1: d1,
							Doc2: d2,
							Score: sim,
							Reason: reason,
						})
					}
				}
			}

			// Sort by score descending
			sort.Slice(duplicates, func(i, j int) bool {
				return duplicates[i].Score > duplicates[j].Score
			})

			if len(duplicates) == 0 {
				fmt.Printf("No duplicates found (threshold %.2f)\n", threshold)
				return nil
			}

			fmt.Printf("Found %d potential duplicate pairs:\n\n", len(duplicates))
			for i, pair := range duplicates {
				fmt.Printf("[%d] Score: %.2f (%s)\n", i+1, pair.Score, pair.Reason)
				fmt.Printf("    Document A: %s\n", truncate(d1Summary(pair.Doc1), 60))
				fmt.Printf("    Document B: %s\n", truncate(d2Summary(pair.Doc2), 60))
				fmt.Println()
			}

			return nil
		},
	}

	cmd.Flags().Float64VarP(&threshold, "threshold", "t", 0.7, "Similarity threshold (0-1, default 0.7)")
	return cmd
}

func metaDoi(d *library.Document) string {
	if doi, ok := d.Meta["doi"].(string); ok && doi != "" {
		return doi
	}
	return ""
}

func d1Summary(d *library.Document) string {
	if d.Title != "" {
		return d.Title
	}
	return d.Path
}

func d2Summary(d *library.Document) string {
	if d.Title != "" {
		return d.Title
	}
	return d.Path
}

type duplicatePair struct {
	Doc1   *library.Document
	Doc2   *library.Document
	Score  float64
	Reason string
}

func titleSimilarity(a, b string) float64 {
	// Normalize: lowercase, remove punctuation, split into tokens
	re := regexp.MustCompile(`[^\w\s]`)
	aClean := re.ReplaceAllString(strings.ToLower(a), "")
	bClean := re.ReplaceAllString(strings.ToLower(b), "")

	setA := make(map[string]bool)
	for _, word := range strings.Fields(aClean) {
		if len(word) > 2 { // ignore very short words
			setA[word] = true
		}
	}
	setB := make(map[string]bool)
	for _, word := range strings.Fields(bClean) {
		if len(word) > 2 {
			setB[word] = true
		}
	}

	// Jaccard similarity: |A ∩ B| / |A ∪ B|
	intersection := 0
	for word := range setA {
		if setB[word] {
			intersection++
		}
	}
	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0.0
	}
	return float64(intersection) / float64(union)
}
