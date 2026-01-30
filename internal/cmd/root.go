// Copyright (c) 2025 Arc Engineering
// SPDX-License-Identifier: MIT

package cmd

import (
	"database/sql"

	"github.com/spf13/cobra"
	"github.com/mtreilly/arc-library/internal/library"
	"github.com/yourorg/arc-sdk/config"
)

// NewRootCmd creates the root command for arc-library.
func NewRootCmd(cfg *config.Config, db *sql.DB) *cobra.Command {
	store, err := library.NewStore(db)
	if err != nil {
		panic(err)
	}

	root := &cobra.Command{
		Use:   "arc-library",
		Short: "Manage your research paper library",
		Long: `Organize, tag, and annotate your research papers.

arc-library provides tools to:
- Import papers from various sources (arxiv, local files)
- Tag and organize papers
- Create collections for projects
- Add annotations and notes
- Search across your library`,
	}

	root.AddCommand(newImportCmd(cfg, store))
	root.AddCommand(newTagCmd(cfg, store))
	root.AddCommand(newCollectionCmd(cfg, store))
	root.AddCommand(newListCmd(cfg, store))
	root.AddCommand(newSearchCmd(cfg, store))
	root.AddCommand(newAnnotateCmd(cfg, store))

	return root
}
