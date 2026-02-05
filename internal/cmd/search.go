// Copyright (c) 2025 Arc Engineering
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"strings"

	"github.com/mtreilly/arc-library/internal/library"
	"github.com/spf13/cobra"
	"github.com/yourorg/arc-sdk/config"
	"github.com/yourorg/arc-sdk/output"
)

func newSearchCmd(cfg *config.Config, store library.LibraryStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search documents and manage saved searches",
		Long:  `Search across document titles, abstracts, and notes. Save common searches for quick access.`,
	}

	cmd.AddCommand(newSearchRunCmd(store))
	cmd.AddCommand(newSearchSaveCmd(store))
	cmd.AddCommand(newSearchListCmd(store))
	cmd.AddCommand(newSearchDeleteCmd(store))

	return cmd
}

func newSearchRunCmd(store library.LibraryStore) *cobra.Command {
	var out output.OutputOptions
	var tag string
	var source string
	var docType string
	var limit int

	cmd := &cobra.Command{
		Use:   "run <query-or-saved-search>",
		Short: "Search documents (or load a saved search)",
		Long: `Search across document titles, abstracts, and notes.
If the argument matches a saved search name, that search is loaded instead.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.Resolve(); err != nil {
				return err
			}

			arg := args[0]

			// Check if it's a saved search
			saved, err := store.GetSavedSearch(arg)
			if err != nil {
				return err
			}

			var opts *library.ListOptions
			if saved != nil {
				// Use saved search
				opts = &library.ListOptions{
					Search: saved.Query,
					Tag:    saved.Tag,
					Source: saved.Source,
					Type:   saved.Type,
					Limit:  limit,
				}
				if tag != "" {
					opts.Tag = tag // Allow overriding
				}
				if source != "" {
					opts.Source = source
				}
				if docType != "" {
					opts.Type = docType
				}
				fmt.Printf("Loaded saved search: %s\n\n", saved.Name)
			} else {
				// Regular search
				opts = &library.ListOptions{
					Search: arg,
					Tag:    tag,
					Source: source,
					Type:   docType,
					Limit:  limit,
				}
			}

			documents, err := store.ListDocuments(opts)
			if err != nil {
				return err
			}

			if len(documents) == 0 {
				if saved != nil {
					fmt.Printf("No documents found for saved search %q\n", saved.Name)
				} else {
					fmt.Printf("No documents found matching %q\n", arg)
				}
				return nil
			}

			if out.Is(output.OutputJSON) {
				return output.JSON(documents)
			}

			queryStr := arg
			if saved != nil {
				queryStr = saved.Query
			}
			fmt.Printf("Found %d result(s) for %q:\n\n", len(documents), queryStr)

			table := output.NewTable("Source ID", "Title", "Tags")
			for _, p := range documents {
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
	cmd.Flags().StringVarP(&source, "source", "s", "", "Filter by source")
	cmd.Flags().StringVar(&docType, "type", "", "Filter by document type")
	cmd.Flags().IntVarP(&limit, "limit", "n", 50, "Limit number of results")

	return cmd
}

func newSearchSaveCmd(store library.LibraryStore) *cobra.Command {
	var name string
	var tag string
	var source string
	var docType string
	var description string

	cmd := &cobra.Command{
		Use:   "save <query>",
		Short: "Save a search for later use",
		Long:  "Save a search query with filters. Use the name to rerun the search later.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]

			if name == "" {
				// Generate name from query
				name = query
				if len(name) > 30 {
					name = name[:30]
				}
				name = strings.ToLower(strings.ReplaceAll(name, " ", "-"))
			}

			// Check for existing
			existing, _ := store.GetSavedSearch(name)
			if existing != nil {
				fmt.Printf("Updating existing saved search: %s\n", name)
			}

			ss := &library.SavedSearch{
				Name:        name,
				Query:       query,
				Tag:         tag,
				Source:      source,
				Type:        docType,
				Description: description,
			}

			if err := store.SaveSearch(ss); err != nil {
				return fmt.Errorf("save search: %w", err)
			}

			fmt.Printf("Search saved as: %s\n", name)
			fmt.Printf("Query: %s\n", query)
			if tag != "" {
				fmt.Printf("Tag filter: %s\n", tag)
			}
			if source != "" {
				fmt.Printf("Source filter: %s\n", source)
			}
			if docType != "" {
				fmt.Printf("Type filter: %s\n", docType)
			}
			fmt.Printf("\nRun with: arc-library search run %s\n", name)

			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Name for this saved search (required)")
	cmd.Flags().StringVarP(&tag, "tag", "t", "", "Filter by tag")
	cmd.Flags().StringVarP(&source, "source", "s", "", "Filter by source")
	cmd.Flags().StringVar(&docType, "type", "", "Filter by document type")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Description")

	cmd.MarkFlagRequired("name")

	return cmd
}

func newSearchListCmd(store library.LibraryStore) *cobra.Command {
	var out output.OutputOptions

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List saved searches",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.Resolve(); err != nil {
				return err
			}

			searches, err := store.ListSavedSearches()
			if err != nil {
				return fmt.Errorf("list searches: %w", err)
			}

			if out.Is(output.OutputJSON) {
				return output.JSON(searches)
			}

			if len(searches) == 0 {
				fmt.Println("No saved searches.")
				fmt.Println("\nSave a search with: arc-library search save <query> --name <name>")
				return nil
			}

			fmt.Printf("Saved searches: %d\n\n", len(searches))

			table := output.NewTable("Name", "Query", "Filters", "Description")
			for _, ss := range searches {
				filters := ""
				if ss.Tag != "" {
					filters += "tag:" + ss.Tag + " "
				}
				if ss.Source != "" {
					filters += "src:" + ss.Source + " "
				}
				if ss.Type != "" {
					filters += "type:" + ss.Type
				}
				if filters == "" {
					filters = "-"
				}

				desc := ss.Description
				if desc == "" {
					desc = "-"
				}

				table.AddRow(ss.Name, truncate(ss.Query, 30), filters, truncate(desc, 25))
			}
			table.Render()

			fmt.Println("\nRun a saved search: arc-library search run <name>")

			return nil
		},
	}

	out.AddOutputFlags(cmd, output.OutputTable)

	return cmd
}

func newSearchDeleteCmd(store library.LibraryStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a saved search",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			ss, err := store.GetSavedSearch(name)
			if err != nil {
				return fmt.Errorf("find search: %w", err)
			}
			if ss == nil {
				return fmt.Errorf("saved search not found: %s", name)
			}

			if err := store.DeleteSavedSearch(ss.ID); err != nil {
				return fmt.Errorf("delete search: %w", err)
			}

			fmt.Printf("Deleted saved search: %s\n", name)
			return nil
		},
	}

	return cmd
}
