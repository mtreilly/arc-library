// Copyright (c) 2025 Arc Engineering
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mtreilly/arc-library/internal/library"
	"github.com/yourorg/arc-sdk/config"
	"github.com/yourorg/arc-sdk/output"
)

func newListCmd(cfg *config.Config, store *library.Store) *cobra.Command {
	var out output.OutputOptions
	var tag string
	var source string
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List papers in the library",
		Long: `List all papers imported into the library.

Examples:
  arc-library list                  # List all papers
  arc-library list --tag ml         # Filter by tag
  arc-library list --source arxiv   # Filter by source
  arc-library list --limit 20       # Limit results`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.Resolve(); err != nil {
				return err
			}

			opts := &library.ListOptions{
				Tag:    tag,
				Source: source,
				Limit:  limit,
			}

			papers, err := store.ListPapers(opts)
			if err != nil {
				return err
			}

			if len(papers) == 0 {
				fmt.Println("No papers found in library.")
				fmt.Println("Use 'arc-library import <path>' to add papers.")
				return nil
			}

			if out.Is(output.OutputJSON) {
				return output.JSON(papers)
			}

			table := output.NewTable("Source ID", "Title", "Tags")
			for _, p := range papers {
				tags := ""
				if len(p.Tags) > 0 {
					tags = strings.Join(p.Tags, ", ")
					if len(tags) > 25 {
						tags = tags[:22] + "..."
					}
				}
				sourceID := p.SourceID
				if sourceID == "" {
					sourceID = p.ID[:8]
				}
				table.AddRow(sourceID, truncate(p.Title, 45), tags)
			}
			table.Render()

			fmt.Printf("\nTotal: %d paper(s)\n", len(papers))
			return nil
		},
	}

	out.AddOutputFlags(cmd, output.OutputTable)
	cmd.Flags().StringVarP(&tag, "tag", "t", "", "Filter by tag")
	cmd.Flags().StringVarP(&source, "source", "s", "", "Filter by source (arxiv, local)")
	cmd.Flags().IntVarP(&limit, "limit", "n", 0, "Limit number of results")

	return cmd
}
