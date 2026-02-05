// Copyright (c) 2025 Arc Engineering
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"

	"github.com/mtreilly/arc-library/internal/cmd"
	"github.com/mtreilly/arc-library/internal/library"
	"github.com/yourorg/arc-sdk/config"
	"github.com/yourorg/arc-sdk/db"
	"github.com/yourorg/arc-sdk/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc-library: failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Storage backend selection via environment variable.
	// Default: "sql" (traditional relational schema).
	// Options: "sql", "kv" (KV store with persistent SQLite), "memory" (in-memory only).
	storage := os.Getenv("ARC_LIBRARY_STORAGE")
	if storage == "" {
		storage = "sql" // default
	}

	var libStore library.LibraryStore

	switch storage {
	case "sql":
		// Traditional arc-library with dedicated schema.
		// If SQLite fails (missing, corrupted, permissions), fall back to in-memory store
		// so the tool remains operational (statelessly) without persistence.
		database, err := db.Open(db.DefaultDBPath())
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: cannot open SQLite database: %v\n", err)
			fmt.Fprintln(os.Stderr, "         falling back to in-memory store (no persistence)")
			kv := store.NewMemoryStore()
			kvStore, err2 := library.NewKVStore(kv)
			if err2 != nil {
				fmt.Fprintf(os.Stderr, "arc-library: failed to init memory KV store: %v\n", err2)
				os.Exit(1)
			}
			libStore = kvStore
			break
		}
		sqlStore, err := library.NewStore(database)
		if err != nil {
			fmt.Fprintf(os.Stderr, "arc-library: failed to init SQL store: %v\n", err)
			os.Exit(1)
		}
		libStore = sqlStore

	case "kv":
		// KV store with persistent SQLite (simpler schema, all JSON)
		kv, err := store.OpenSQLiteStore("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "arc-library: failed to open KV SQLite: %v\n", err)
			os.Exit(1)
		}
		kvStore, err := library.NewKVStore(kv)
		if err != nil {
			fmt.Fprintf(os.Stderr, "arc-library: failed to init KV store: %v\n", err)
			os.Exit(1)
		}
		libStore = kvStore

	case "memory":
		// In-memory only - degrades gracefully, no persistence
		kv := store.NewMemoryStore()
		kvStore, err := library.NewKVStore(kv)
		if err != nil {
			fmt.Fprintf(os.Stderr, "arc-library: failed to init memory store: %v\n", err)
			os.Exit(1)
		}
		libStore = kvStore

	default:
		fmt.Fprintf(os.Stderr, "arc-library: unknown storage backend %q (choose sql, kv, or memory)\n", storage)
		os.Exit(1)
	}

	root := cmd.NewRootCmd(cfg, libStore)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
