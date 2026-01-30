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

func newSearchCmd(cfg *config.Config, store *library.Store) *cobra.Command {
	var out output.OutputOptions
	var tag string
	var limit int

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search papers in the library",
		Long: `Search across paper titles, abstracts, and notes.

Examples:
  arc-library search "transformer"        # Search all fields
  arc-library search "attention" --tag ml # Search with tag filter
  arc-library search "neural" --limit 10  # Limit results`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.Resolve(); err != nil {
				return err
			}

			query := args[0]

			opts := &library.ListOptions{
				Search: query,
				Tag:    tag,
				Limit:  limit,
			}

			papers, err := store.ListPapers(opts)
			if err != nil {
				return err
			}

			if len(papers) == 0 {
				fmt.Printf("No papers found matching %q\n", query)
				return nil
			}

			if out.Is(output.OutputJSON) {
				return output.JSON(papers)
			}

			fmt.Printf("Found %d result(s) for %q:\n\n", len(papers), query)

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

			return nil
		},
	}

	out.AddOutputFlags(cmd, output.OutputTable)
	cmd.Flags().StringVarP(&tag, "tag", "t", "", "Also filter by tag")
	cmd.Flags().IntVarP(&limit, "limit", "n", 50, "Limit number of results")

	return cmd
}
