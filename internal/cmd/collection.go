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

func newCollectionCmd(cfg *config.Config, store library.LibraryStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "collection",
		Aliases: []string{"coll", "c"},
		Short:   "Manage paper collections",
		Long:    `Create, list, and manage collections of papers.`,
	}

	cmd.AddCommand(newCollectionCreateCmd(store))
	cmd.AddCommand(newCollectionListCmd(store))
	cmd.AddCommand(newCollectionShowCmd(store))
	cmd.AddCommand(newCollectionAddCmd(store))
	cmd.AddCommand(newCollectionRemoveCmd(store))
	cmd.AddCommand(newCollectionDeleteCmd(store))

	return cmd
}

func newCollectionCreateCmd(store library.LibraryStore) *cobra.Command {
	var description string

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new collection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			existing, _ := store.GetCollection(name)
			if existing != nil {
				return fmt.Errorf("collection %q already exists", name)
			}

			c, err := store.CreateCollection(name, description)
			if err != nil {
				return err
			}

			fmt.Printf("Created collection: %s (id: %s)\n", c.Name, c.ID)
			return nil
		},
	}

	cmd.Flags().StringVarP(&description, "description", "d", "", "Collection description")

	return cmd
}

func newCollectionListCmd(store library.LibraryStore) *cobra.Command {
	var out output.OutputOptions

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all collections",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.Resolve(); err != nil {
				return err
			}

			collections, err := store.ListCollections()
			if err != nil {
				return err
			}

			if len(collections) == 0 {
				fmt.Println("No collections found.")
				return nil
			}

			if out.Is(output.OutputJSON) {
				return output.JSON(collections)
			}

			table := output.NewTable("Name", "Papers", "Description")
			for _, c := range collections {
				desc := truncate(c.Description, 40)
				table.AddRow(c.Name, fmt.Sprintf("%d", len(c.PaperIDs)), desc)
			}
			table.Render()

			return nil
		},
	}

	out.AddOutputFlags(cmd, output.OutputTable)

	return cmd
}

func newCollectionShowCmd(store library.LibraryStore) *cobra.Command {
	var out output.OutputOptions

	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Show papers in a collection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.Resolve(); err != nil {
				return err
			}

			c, err := store.GetCollection(args[0])
			if err != nil {
				return err
			}
			if c == nil {
				return fmt.Errorf("collection not found: %s", args[0])
			}

			fmt.Printf("Collection: %s\n", c.Name)
			if c.Description != "" {
				fmt.Printf("Description: %s\n", c.Description)
			}
			fmt.Printf("Papers: %d\n\n", len(c.PaperIDs))

			if len(c.PaperIDs) == 0 {
				return nil
			}

			if out.Is(output.OutputJSON) {
				var papers []*library.Paper
				for _, id := range c.PaperIDs {
					p, _ := store.GetPaper(id)
					if p != nil {
						papers = append(papers, p)
					}
				}
				return output.JSON(papers)
			}

			table := output.NewTable("Source ID", "Title", "Tags")
			for _, id := range c.PaperIDs {
				p, err := store.GetPaper(id)
				if err != nil || p == nil {
					continue
				}
				tags := ""
				if len(p.Tags) > 0 {
					tags = truncate(fmt.Sprintf("%v", p.Tags), 20)
				}
				table.AddRow(p.SourceID, truncate(p.Title, 45), tags)
			}
			table.Render()

			return nil
		},
	}

	out.AddOutputFlags(cmd, output.OutputTable)

	return cmd
}

func newCollectionAddCmd(store library.LibraryStore) *cobra.Command {
	return &cobra.Command{
		Use:   "add <collection> <paper-id> [paper-id...]",
		Short: "Add papers to a collection",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			collName := args[0]
			paperIDs := args[1:]

			c, err := store.GetCollection(collName)
			if err != nil {
				return err
			}
			if c == nil {
				return fmt.Errorf("collection not found: %s", collName)
			}

			added := 0
			for _, pid := range paperIDs {
				paper, _ := store.GetPaper(pid)
				if paper == nil {
					// Try to find by source ID
					papers, _ := store.ListPapers(&library.ListOptions{Search: pid, Limit: 1})
					if len(papers) > 0 {
						paper = papers[0]
					}
				}
				if paper == nil {
					fmt.Printf("Paper not found: %s\n", pid)
					continue
				}

				if err := store.AddToCollection(c.ID, paper.ID); err != nil {
					fmt.Printf("Failed to add %s: %v\n", pid, err)
					continue
				}
				fmt.Printf("Added: %s\n", truncate(paper.Title, 50))
				added++
			}

			fmt.Printf("\nAdded %d paper(s) to %s.\n", added, c.Name)
			return nil
		},
	}
}

func newCollectionRemoveCmd(store library.LibraryStore) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <collection> <paper-id> [paper-id...]",
		Short: "Remove papers from a collection",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			collName := args[0]
			paperIDs := args[1:]

			c, err := store.GetCollection(collName)
			if err != nil {
				return err
			}
			if c == nil {
				return fmt.Errorf("collection not found: %s", collName)
			}

			removed := 0
			for _, pid := range paperIDs {
				paper, _ := store.GetPaper(pid)
				if paper == nil {
					papers, _ := store.ListPapers(&library.ListOptions{Search: pid, Limit: 1})
					if len(papers) > 0 {
						paper = papers[0]
					}
				}
				if paper == nil {
					fmt.Printf("Paper not found: %s\n", pid)
					continue
				}

				if err := store.RemoveFromCollection(c.ID, paper.ID); err != nil {
					fmt.Printf("Failed to remove %s: %v\n", pid, err)
					continue
				}
				fmt.Printf("Removed: %s\n", truncate(paper.Title, 50))
				removed++
			}

			fmt.Printf("\nRemoved %d paper(s) from %s.\n", removed, c.Name)
			return nil
		},
	}
}

func newCollectionDeleteCmd(store library.LibraryStore) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a collection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := store.GetCollection(args[0])
			if err != nil {
				return err
			}
			if c == nil {
				return fmt.Errorf("collection not found: %s", args[0])
			}

			if !force && len(c.PaperIDs) > 0 {
				return fmt.Errorf("collection %q has %d papers, use --force to delete", c.Name, len(c.PaperIDs))
			}

			if err := store.DeleteCollection(c.ID); err != nil {
				return err
			}

			fmt.Printf("Deleted collection: %s\n", c.Name)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Delete even if collection has papers")

	return cmd
}
