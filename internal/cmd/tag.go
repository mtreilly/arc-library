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
		Short: "Manage paper tags",
		Long:  `Add, remove, and list tags on papers.`,
	}

	cmd.AddCommand(newTagAddCmd(store))
	cmd.AddCommand(newTagRemoveCmd(store))
	cmd.AddCommand(newTagListCmd(store))

	return cmd
}

func newTagAddCmd(store library.LibraryStore) *cobra.Command {
	return &cobra.Command{
		Use:   "add <paper-id> <tag> [tag...]",
		Short: "Add tags to a paper",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			paperID := args[0]
			tags := args[1:]

			// Try to find paper by ID or source ID
			paper, err := store.GetPaper(paperID)
			if err != nil {
				return err
			}
			if paper == nil {
				// Try by source ID
				papers, _ := store.ListPapers(&library.ListOptions{Search: paperID, Limit: 1})
				if len(papers) > 0 {
					paper = papers[0]
				}
			}
			if paper == nil {
				return fmt.Errorf("paper not found: %s", paperID)
			}

			for _, tag := range tags {
				if err := store.AddTag(paper.ID, tag); err != nil {
					return fmt.Errorf("add tag %q: %w", tag, err)
				}
				fmt.Printf("Added tag %q to %s\n", tag, truncate(paper.Title, 40))
			}

			return nil
		},
	}
}

func newTagRemoveCmd(store library.LibraryStore) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <paper-id> <tag> [tag...]",
		Short: "Remove tags from a paper",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			paperID := args[0]
			tags := args[1:]

			paper, err := store.GetPaper(paperID)
			if err != nil {
				return err
			}
			if paper == nil {
				papers, _ := store.ListPapers(&library.ListOptions{Search: paperID, Limit: 1})
				if len(papers) > 0 {
					paper = papers[0]
				}
			}
			if paper == nil {
				return fmt.Errorf("paper not found: %s", paperID)
			}

			for _, tag := range tags {
				if err := store.RemoveTag(paper.ID, tag); err != nil {
					return fmt.Errorf("remove tag %q: %w", tag, err)
				}
				fmt.Printf("Removed tag %q from %s\n", tag, truncate(paper.Title, 40))
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

			table := output.NewTable("Tag", "Papers")
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
