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

func newAnnotateCmd(cfg *config.Config, store *library.Store) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "annotate",
		Aliases: []string{"ann", "note"},
		Short:   "Manage paper annotations",
		Long:    `Add, list, and remove annotations on papers.`,
	}

	cmd.AddCommand(newAnnotateAddCmd(store))
	cmd.AddCommand(newAnnotateListCmd(store))
	cmd.AddCommand(newAnnotateDeleteCmd(store))

	return cmd
}

func newAnnotateAddCmd(store *library.Store) *cobra.Command {
	var page int
	var annType string
	var color string

	cmd := &cobra.Command{
		Use:   "add <paper-id> <content>",
		Short: "Add an annotation to a paper",
		Long: `Add a note or highlight to a paper.

Examples:
  arc-library annotate add 2304.00067 "Key insight about attention"
  arc-library annotate add 2304.00067 "Important formula" --page 5
  arc-library annotate add 2304.00067 "TODO: follow up" --type bookmark`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			paperID := args[0]
			content := args[1]

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

			ann := &library.Annotation{
				PaperID: paper.ID,
				Type:    annType,
				Content: content,
				Page:    page,
				Color:   color,
			}

			if err := store.AddAnnotation(ann); err != nil {
				return fmt.Errorf("add annotation: %w", err)
			}

			fmt.Printf("Added %s to %s", annType, truncate(paper.Title, 40))
			if page > 0 {
				fmt.Printf(" (page %d)", page)
			}
			fmt.Println()

			return nil
		},
	}

	cmd.Flags().IntVarP(&page, "page", "p", 0, "Page number")
	cmd.Flags().StringVarP(&annType, "type", "t", "note", "Type: note, highlight, bookmark")
	cmd.Flags().StringVarP(&color, "color", "c", "", "Highlight color")

	return cmd
}

func newAnnotateListCmd(store *library.Store) *cobra.Command {
	var out output.OutputOptions

	cmd := &cobra.Command{
		Use:   "list <paper-id>",
		Short: "List annotations for a paper",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.Resolve(); err != nil {
				return err
			}

			paperID := args[0]

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

			annotations, err := store.GetAnnotations(paper.ID)
			if err != nil {
				return err
			}

			if len(annotations) == 0 {
				fmt.Printf("No annotations for %s\n", truncate(paper.Title, 50))
				return nil
			}

			fmt.Printf("Annotations for: %s\n\n", truncate(paper.Title, 50))

			if out.Is(output.OutputJSON) {
				return output.JSON(annotations)
			}

			table := output.NewTable("Type", "Page", "Content", "Created")
			for _, a := range annotations {
				pageStr := "-"
				if a.Page > 0 {
					pageStr = fmt.Sprintf("%d", a.Page)
				}
				content := truncate(a.Content, 40)
				created := a.CreatedAt.Format("2006-01-02")
				table.AddRow(a.Type, pageStr, content, created)
			}
			table.Render()

			return nil
		},
	}

	out.AddOutputFlags(cmd, output.OutputTable)

	return cmd
}

func newAnnotateDeleteCmd(store *library.Store) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <annotation-id>",
		Short: "Delete an annotation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := store.DeleteAnnotation(args[0]); err != nil {
				return err
			}
			fmt.Println("Annotation deleted.")
			return nil
		},
	}
}
