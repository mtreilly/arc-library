// Copyright (c) 2025 Arc Engineering
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mtreilly/arc-library/internal/library"
	"github.com/yourorg/arc-sdk/config"
	"github.com/yourorg/arc-sdk/output"
)

func newSessionCmd(cfg *config.Config, store library.LibraryStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Manage reading sessions",
		Long:  "Track time spent reading documents",
	}

	cmd.AddCommand(newSessionStartCmd(store))
	cmd.AddCommand(newSessionEndCmd(store))
	cmd.AddCommand(newSessionListCmd(store))

	return cmd
}

func newSessionStartCmd(store library.LibraryStore) *cobra.Command {
	var out output.OutputOptions

	cmd := &cobra.Command{
		Use:   "start <document-id>",
		Short: "Start a reading session for a document",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.Resolve(); err != nil {
				return err
			}

			docID := args[0]

			// Verify document exists
			doc, err := store.GetDocument(docID)
			if err != nil {
				return err
			}
			if doc == nil {
				return fmt.Errorf("document not found: %s", docID)
			}

			session, err := store.StartSession(docID)
			if err != nil {
				return fmt.Errorf("start session: %w", err)
			}

			if out.Is(output.OutputJSON) {
				return output.JSON(session)
			}

			fmt.Printf("Session started: %s\n", session.ID)
			fmt.Printf("Document: %s - %s\n", docID, truncate(doc.Title, 50))
			fmt.Printf("Started at: %s\n", session.StartAt.Format(time.RFC3339))
			return nil
		},
	}

	out.AddOutputFlags(cmd, output.OutputJSON)
	return cmd
}

func newSessionEndCmd(store library.LibraryStore) *cobra.Command {
	var (
		pages int
		notes string
		out   output.OutputOptions
	)

	cmd := &cobra.Command{
		Use:   "end <session-id>",
		Short: "End a reading session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.Resolve(); err != nil {
				return err
			}

			sessionID := args[0]
			if err := store.EndSession(sessionID, pages, notes); err != nil {
				return fmt.Errorf("end session: %w", err)
			}

			fmt.Printf("Session ended: %s\n", sessionID)
			if pages > 0 {
				fmt.Printf("Pages read: %d\n", pages)
			}
			if notes != "" {
				fmt.Printf("Notes: %s\n", notes)
			}
			return nil
		},
	}

	cmd.Flags().IntVarP(&pages, "pages", "p", 0, "Number of pages read")
	cmd.Flags().StringVarP(&notes, "notes", "n", "", "Session notes")
	out.AddOutputFlags(cmd, output.OutputJSON)
	return cmd
}

func newSessionListCmd(store library.LibraryStore) *cobra.Command {
	var (
		documentID string
		limit      int
		out        output.OutputOptions
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List reading sessions",
		Long:  "List sessions, optionally filtered by document.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.Resolve(); err != nil {
				return err
			}

			var sessions []*library.ReadingSession
			var err error

			if documentID != "" {
				sessions, err = store.ListSessions(documentID)
				if err != nil {
					return fmt.Errorf("list sessions: %w", err)
				}
			} else {
				// Global list: aggregate all documents' sessions
				docs, err := store.ListDocuments(nil)
				if err != nil {
					return fmt.Errorf("list documents: %w", err)
				}
				var all []*library.ReadingSession
				for _, d := range docs {
					sess, err := store.ListSessions(d.ID)
					if err != nil {
						continue
					}
					all = append(all, sess...)
				}
				// Sort by start time descending (simple bubble sort)
				for i := 1; i < len(all); i++ {
					j := i
					for j > 0 && all[j-1].StartAt.Before(all[j].StartAt) {
						all[j-1], all[j] = all[j], all[j-1]
						j--
					}
				}
				sessions = all
			}

			// Apply limit
			if limit > 0 && len(sessions) > limit {
				sessions = sessions[:limit]
			}

			if out.Is(output.OutputJSON) {
				return output.JSON(sessions)
			}

			if len(sessions) == 0 {
				fmt.Println("No sessions found.")
				return nil
			}

			table := output.NewTable("Session ID", "Document ID", "Start", "End", "Pages", "Notes")
			for _, s := range sessions {
				start := s.StartAt.Format("2006-01-02 15:04")
				end := ""
				if !s.EndAt.IsZero() {
					end = s.EndAt.Format("15:04")
				}
				notes := truncate(s.Notes, 20)
				table.AddRow(s.ID, truncate(s.DocumentID, 8), start, end, fmt.Sprintf("%d", s.PagesRead), notes)
			}
			table.Render()

			fmt.Printf("\nTotal: %d session(s)\n", len(sessions))
			return nil
		},
	}

	cmd.Flags().StringVarP(&documentID, "document", "d", "", "Filter sessions by document ID")
	cmd.Flags().IntVarP(&limit, "limit", "n", 0, "Limit number of results")
	out.AddOutputFlags(cmd, output.OutputTable)
	return cmd
}
