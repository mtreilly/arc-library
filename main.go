// Copyright (c) 2025 Arc Engineering
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"

	"github.com/mtreilly/arc-library/internal/cmd"
	"github.com/yourorg/arc-sdk/config"
	"github.com/yourorg/arc-sdk/db"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc-library: failed to load config: %v\n", err)
		os.Exit(1)
	}

	database, err := db.Open(db.DefaultDBPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc-library: failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	root := cmd.NewRootCmd(cfg, database)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
