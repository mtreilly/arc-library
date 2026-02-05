// Copyright (c) 2025 Arc Engineering
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/spf13/cobra"
	"github.com/mtreilly/arc-library/internal/library"
	"github.com/yourorg/arc-sdk/config"
)

// NewRootCmd creates the root command for arc-library.
func NewRootCmd(cfg *config.Config, store library.LibraryStore) *cobra.Command {

	root := &cobra.Command{
		Use:   "arc-library",
		Short: "Manage your research document library",
		Long: `Organize, tag, and annotate your research documents.

arc-library provides tools to:
- Import documents from various sources (arxiv, local files)
- Tag and organize documents
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
	root.AddCommand(newSessionCmd(cfg, store))
	root.AddCommand(newStatsCmd(cfg, store))
	root.AddCommand(newFlashcardCmd(cfg, store))
	root.AddCommand(newExportCmd(cfg, store))
	root.AddCommand(newAICmd(cfg, store))

	return root
}
