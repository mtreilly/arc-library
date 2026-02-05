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

func newFlashcardCmd(cfg *config.Config, store library.LibraryStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "flashcard",
		Short: "Manage spaced repetition flashcards",
		Long:  "Create, review, and manage flashcards for active recall learning",
	}

	cmd.AddCommand(newFlashcardAddCmd(store))
	cmd.AddCommand(newFlashcardListCmd(store))
	cmd.AddCommand(newFlashcardReviewCmd(store))
	cmd.AddCommand(newFlashcardDeleteCmd(store))
	cmd.AddCommand(newFlashcardDueCmd(store))

	return cmd
}

func newFlashcardAddCmd(store library.LibraryStore) *cobra.Command {
	var (
		docID  string
		fType  string
		front  string
		back   string
		cloze  string
		tags   []string
		due    int // days from now
		out    output.OutputOptions
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new flashcard",
		Long:  "Create a flashcard from a document. Can be basic (front/back) or cloze deletion.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.Resolve(); err != nil {
				return err
			}

			if front == "" {
				return fmt.Errorf("front text is required")
			}

			// Verify document exists
			if docID != "" {
				doc, err := store.GetDocument(docID)
				if err != nil {
					return err
				}
				if doc == nil {
					return fmt.Errorf("document not found: %s", docID)
				}
			}

			card := &library.Flashcard{
				DocumentID: docID,
				Type:       fType,
				Front:      front,
				Back:       back,
				Cloze:      cloze,
				Tags:       tags,
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}

			// Set initial due date
			if due == 0 {
				due = 1 // default due tomorrow
			}
			card.DueAt = time.Now().AddDate(0, 0, due)
			card.Interval = 0
			card.Ease = 2.5

			if err := store.AddFlashcard(card); err != nil {
				return fmt.Errorf("add flashcard: %w", err)
			}

			if out.Is(output.OutputJSON) {
				return output.JSON(card)
			}

			fmt.Printf("Flashcard created: %s\n", card.ID)
			fmt.Printf("Front: %s\n", truncate(card.Front, 60))
			if card.Type == "cloze" {
				fmt.Printf("Cloze: %s\n", card.Cloze)
			} else {
				fmt.Printf("Back: %s\n", truncate(card.Back, 60))
			}
			fmt.Printf("Due: %s\n", card.DueAt.Format("2006-01-02"))
			return nil
		},
	}

	cmd.Flags().StringVarP(&docID, "document", "d", "", "Document ID (optional)")
	cmd.Flags().StringVarP(&fType, "type", "t", "basic", "Card type: basic or cloze")
	cmd.Flags().StringVar(&front, "front", "", "Front side text (required)")
	cmd.Flags().StringVar(&back, "back", "", "Back side text (for basic cards)")
	cmd.Flags().StringVar(&cloze, "cloze", "", "Cloze deletion text (e.g., 'The capital of France is {{c1::Paris}}')")
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "Tags")
	cmd.Flags().IntVar(&due, "due", 1, "Days until due (default: 1)")
	out.AddOutputFlags(cmd, output.OutputJSON)

	return cmd
}

func newFlashcardListCmd(store library.LibraryStore) *cobra.Command {
	var (
		docID    string
		tag      string
		due      bool
		limit    int
		out      output.OutputOptions
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List flashcards",
		Long:  "List all flashcards, optionally filtered by document, tag, or due status.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.Resolve(); err != nil {
				return err
			}

			opts := &library.FlashcardListOptions{
				DocumentID: docID,
				Tag:        tag,
				Due:        due,
				Limit:      limit,
			}

			cards, err := store.ListFlashcards(opts)
			if err != nil {
				return fmt.Errorf("list flashcards: %w", err)
			}

			if out.Is(output.OutputJSON) {
				return output.JSON(cards)
			}

			if len(cards) == 0 {
				fmt.Println("No flashcards found.")
				return nil
			}

			table := output.NewTable("ID", "Type", "Front", "Due", "Interval", "Ease")
			for _, c := range cards {
				front := truncate(c.Front, 30)
				dueStr := ""
				if !c.DueAt.IsZero() {
					dueStr = c.DueAt.Format("2006-01-02")
					if c.DueAt.Before(time.Now()) {
						dueStr += " (!)"
					}
				}
				table.AddRow(truncate(c.ID, 8), c.Type, front, dueStr, fmt.Sprintf("%d", c.Interval), fmt.Sprintf("%.2f", c.Ease))
			}
			table.Render()

			fmt.Printf("\nTotal: %d flashcard(s)\n", len(cards))
			return nil
		},
	}

	cmd.Flags().StringVarP(&docID, "document", "d", "", "Filter by document ID")
	cmd.Flags().StringVarP(&tag, "tag", "t", "", "Filter by tag")
	cmd.Flags().BoolVar(&due, "due", false, "Show only due cards")
	cmd.Flags().IntVarP(&limit, "limit", "n", 0, "Limit number of results")
	out.AddOutputFlags(cmd, output.OutputTable)

	return cmd
}

func newFlashcardReviewCmd(store library.LibraryStore) *cobra.Command {
	var (
		quality int
		out     output.OutputOptions
	)

	cmd := &cobra.Command{
		Use:   "review <flashcard-id>",
		Short: "Review a flashcard (rate your recall)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.Resolve(); err != nil {
				return err
			}

			if quality < 0 || quality > 5 {
				return fmt.Errorf("quality must be between 0 and 5")
			}

			card, err := store.ReviewFlashcard(args[0], quality)
			if err != nil {
				return fmt.Errorf("review flashcard: %w", err)
			}

			if out.Is(output.OutputJSON) {
				return output.JSON(card)
			}

			fmt.Printf("Flashcard reviewed: %s\n", card.ID)
			fmt.Printf("Quality: %d/5\n", quality)
			fmt.Printf("New interval: %d days\n", card.Interval)
			fmt.Printf("New ease: %.2f\n", card.Ease)
			fmt.Printf("Next due: %s\n", card.DueAt.Format("2006-01-02"))
			return nil
		},
	}

	cmd.Flags().IntVarP(&quality, "quality", "q", 4, "Quality rating 0-5 (0=blackout, 5=perfect)")
	out.AddOutputFlags(cmd, output.OutputJSON)

	return cmd
}

func newFlashcardDeleteCmd(store library.LibraryStore) *cobra.Command {
	var out output.OutputOptions
	cmd := &cobra.Command{
		Use:   "delete <flashcard-id>",
		Short: "Delete a flashcard",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.Resolve(); err != nil {
				return err
			}

			id := args[0]
			if err := store.DeleteFlashcard(id); err != nil {
				return fmt.Errorf("delete flashcard: %w", err)
			}

			fmt.Printf("Flashcard deleted: %s\n", id)
			return nil
		},
	}

	out.AddOutputFlags(cmd, output.OutputJSON)
	return cmd
}

func newFlashcardDueCmd(store library.LibraryStore) *cobra.Command {
	var (
		limit int
		out   output.OutputOptions
	)

	cmd := &cobra.Command{
		Use:   "due",
		Short: "List due flashcards for review",
		Long:  "Show all flashcards that are due for review today.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.Resolve(); err != nil {
				return err
			}

			now := time.Now()
			cards, err := store.GetDueFlashcards(now)
			if err != nil {
				return fmt.Errorf("get due flashcards: %w", err)
			}

			if limit > 0 && len(cards) > limit {
				cards = cards[:limit]
			}

			if out.Is(output.OutputJSON) {
				return output.JSON(cards)
			}

			if len(cards) == 0 {
				fmt.Println("No flashcards due today!")
				return nil
			}

			fmt.Printf("You have %d flashcard(s) due for review:\n\n", len(cards))

			table := output.NewTable("ID", "Document", "Front", "Interval", "Due")
			for _, c := range cards {
				// Get document title
				doc, _ := store.GetDocument(c.DocumentID)
				docTitle := ""
				if doc != nil {
					docTitle = truncate(doc.Title, 20)
				}
				front := truncate(c.Front, 30)
				dueStr := c.DueAt.Format("2006-01-02")
				if c.DueAt.Before(now) {
					dueStr += " (overdue)"
				}
				table.AddRow(truncate(c.ID, 8), docTitle, front, fmt.Sprintf("%d", c.Interval), dueStr)
			}
			table.Render()

			fmt.Printf("\nReview with: arc-library flashcard review <id> --quality <0-5>\n")
			return nil
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "n", 0, "Limit number of cards shown")
	out.AddOutputFlags(cmd, output.OutputJSON)

	return cmd
}
