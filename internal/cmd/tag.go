// Copyright (c) 2025 Arc Engineering
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/mtreilly/arc-library/internal/library"
	"github.com/yourorg/arc-sdk/config"
	"github.com/yourorg/arc-sdk/output"
)

func newTagCmd(cfg *config.Config, store library.LibraryStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag",
		Short: "Manage document tags",
		Long:  `Add, remove, and list tags on documents.`,
	}

	cmd.AddCommand(newTagAddCmd(store))
	cmd.AddCommand(newTagRemoveCmd(store))
	cmd.AddCommand(newTagListCmd(store))

	return cmd
}

func newTagAddCmd(store library.LibraryStore) *cobra.Command {
	return &cobra.Command{
		Use:   "add <document-id> <tag> [tag...]",
		Short: "Add tags to a document",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			documentID := args[0]
			tags := args[1:]

			// Try to find document by ID or source ID
			document, err := store.GetDocument(documentID)
			if err != nil {
				return err
			}
			if document == nil {
				// Try by source ID
				documents, _ := store.ListDocuments(&library.ListOptions{Search: documentID, Limit: 1})
				if len(documents) > 0 {
					document = documents[0]
				}
			}
			if document == nil {
				return fmt.Errorf("document not found: %s", documentID)
			}

			for _, tag := range tags {
				if err := store.AddTag(document.ID, tag); err != nil {
					return fmt.Errorf("add tag %q: %w", tag, err)
				}
				fmt.Printf("Added tag %q to %s\n", tag, truncate(document.Title, 40))
			}

			return nil
		},
	}
}

func newTagRemoveCmd(store library.LibraryStore) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <document-id> <tag> [tag...]",
		Short: "Remove tags from a document",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			documentID := args[0]
			tags := args[1:]

			document, err := store.GetDocument(documentID)
			if err != nil {
				return err
			}
			if document == nil {
				documents, _ := store.ListDocuments(&library.ListOptions{Search: documentID, Limit: 1})
				if len(documents) > 0 {
					document = documents[0]
				}
			}
			if document == nil {
				return fmt.Errorf("document not found: %s", documentID)
			}

			for _, tag := range tags {
				if err := store.RemoveTag(document.ID, tag); err != nil {
					return fmt.Errorf("remove tag %q: %w", tag, err)
				}
				fmt.Printf("Removed tag %q from %s\n", tag, truncate(document.Title, 40))
			}

			return nil
		},
	}
}

func newTagListCmd(store library.LibraryStore) *cobra.Command {
	var out output.OutputOptions

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all tags",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.Resolve(); err != nil {
				return err
			}

			tags, err := store.ListTags()
			if err != nil {
				return err
			}

			if len(tags) == 0 {
				fmt.Println("No tags found.")
				return nil
			}

			if out.Is(output.OutputJSON) {
				return output.JSON(tags)
			}

			// Sort by count descending
			type tagCount struct {
				Tag   string
				Count int
			}
			var sorted []tagCount
			for tag, count := range tags {
				sorted = append(sorted, tagCount{tag, count})
			}
			sort.Slice(sorted, func(i, j int) bool {
				return sorted[i].Count > sorted[j].Count
			})

			table := output.NewTable("Tag", "Documents")
			for _, tc := range sorted {
				table.AddRow(tc.Tag, fmt.Sprintf("%d", tc.Count))
			}
			table.Render()

			return nil
		},
	}

	out.AddOutputFlags(cmd, output.OutputTable)

	return cmd
}
