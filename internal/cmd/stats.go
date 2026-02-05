// Copyright (c) 2025 Arc Engineering
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mtreilly/arc-library/internal/library"
	"github.com/yourorg/arc-sdk/config"
	"github.com/yourorg/arc-sdk/output"
)

func newStatsCmd(cfg *config.Config, store library.LibraryStore) *cobra.Command {
	var out output.OutputOptions

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show library statistics",
		Long:  `Display statistics about your library: document counts, tag cloud, etc.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.Resolve(); err != nil {
				return err
			}

			// Get all documents (no filter)
			docs, err := store.ListDocuments(nil)
			if err != nil {
				return err
			}

			// Count by type
			typeCounts := make(map[library.DocumentType]int)
			for _, d := range docs {
				typeCounts[d.Type]++
			}

			// Total tags
			tagCounts, err := store.ListTags()
			if err != nil {
				return err
			}

			// Collections count
			collections, err := store.ListCollections()
			if err != nil {
				return err
			}

			// Annotations count: sum over all documents
			totalAnnotations := 0
			for _, d := range docs {
				anns, _ := store.GetAnnotations(d.ID)
				totalAnnotations += len(anns)
			}

			// Reading sessions total
			totalSessions := 0
			totalPagesRead := 0
			for _, d := range docs {
				sessions, _ := store.ListSessions(d.ID)
				totalSessions += len(sessions)
				for _, s := range sessions {
					totalPagesRead += s.PagesRead
				}
			}

			if out.Is(output.OutputJSON) {
				stats := map[string]any{
					"documents":          len(docs),
					"by_type":            typeCounts,
					"tags":               tagCounts,
					"collections":        len(collections),
					"annotations":        totalAnnotations,
					"reading_sessions":   totalSessions,
					"pages_read":         totalPagesRead,
				}
				return output.JSON(stats)
			}

			fmt.Printf("Library Statistics\n")
			fmt.Printf("==================\n\n")
			fmt.Printf("Documents:     %d\n", len(docs))
			fmt.Println("By type:")
			for t, c := range typeCounts {
				fmt.Printf("  %s: %d\n", t, c)
			}
			fmt.Printf("Tags:          %d unique\n", len(tagCounts))
			fmt.Printf("Collections:   %d\n", len(collections))
			fmt.Printf("Annotations:   %d\n", totalAnnotations)
			fmt.Printf("Reading sessions: %d\n", totalSessions)
			fmt.Printf("Pages read:    %d\n", totalPagesRead)

			return nil
		},
	}

	out.AddOutputFlags(cmd, output.OutputTable)
	return cmd
}
