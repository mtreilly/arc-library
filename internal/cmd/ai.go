// Copyright (c) 2025 Arc Engineering
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/mtreilly/arc-library/internal/library"
	"github.com/spf13/cobra"
	"github.com/yourorg/arc-sdk/config"
)

func newAICmd(cfg *config.Config, store library.LibraryStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ai",
		Short: "AI-powered document analysis",
		Long:  "Use arc-ai to generate summaries and answer questions about your documents.",
	}

	cmd.AddCommand(newAISummaryCmd(store))
	cmd.AddCommand(newAIQnACmd(store))
	cmd.AddCommand(newAIFlashcardsCmd(store))

	return cmd
}

func newAISummaryCmd(store library.LibraryStore) *cobra.Command {
	var (
		length   int
		storeRes bool
	)

	cmd := &cobra.Command{
		Use:   "summary <document-id>",
		Short: "Generate a summary of a document",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			docID := args[0]
			doc, err := store.GetDocument(docID)
			if err != nil {
				return fmt.Errorf("get document: %w", err)
			}
			if doc == nil {
				return fmt.Errorf("document not found: %s", docID)
			}

			// Build context
			var context strings.Builder
			context.WriteString(fmt.Sprintf("Title: %s\n", doc.Title))
			if len(doc.Authors) > 0 {
				context.WriteString(fmt.Sprintf("Authors: %s\n", strings.Join(doc.Authors, ", ")))
			}
			if doc.Abstract != "" {
				context.WriteString(fmt.Sprintf("Abstract: %s\n", doc.Abstract))
			}
			if doc.FullText != "" {
				text := doc.FullText
				if len(text) > 4000 {
					text = text[:4000] + "... (truncated)"
				}
				context.WriteString(fmt.Sprintf("\nFull Text:\n%s\n", text))
			}

			prompt := "Provide a comprehensive summary of this document, highlighting the key contributions and findings."
			if length > 0 {
				prompt += fmt.Sprintf(" Keep the summary to approximately %d words.", length)
			}

			// Pipe context to arc-ai ask
			aiCmd := exec.Command("arc-ai", "ask", prompt)
			aiCmd.Stdin = strings.NewReader(context.String())
			var out bytes.Buffer
			aiCmd.Stdout = &out
			aiCmd.Stderr = &out
			if err := aiCmd.Run(); err != nil {
				return fmt.Errorf("arc-ai failed: %w\nOutput: %s", err, out.String())
			}

			fmt.Println("=== AI Summary ===")
			fmt.Println(out.String())
			fmt.Println()

			if storeRes {
				if doc.Meta == nil {
					doc.Meta = make(map[string]any)
				}
				doc.Meta["ai_summary"] = out.String()
				if err := store.UpdateDocument(doc); err != nil {
					return fmt.Errorf("store summary: %w", err)
				}
				fmt.Println("Summary stored in document metadata.")
			}

			return nil
		},
	}

	cmd.Flags().IntVarP(&length, "length", "l", 200, "Target summary length in words")
	cmd.Flags().BoolVarP(&storeRes, "store", "s", false, "Store the summary in the document")
	return cmd
}

func newAIQnACmd(store library.LibraryStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "qna <document-id> <question>",
		Short: "Ask a question about a document",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			docID := args[0]
			question := strings.Join(args[1:], " ")

			doc, err := store.GetDocument(docID)
			if err != nil {
				return fmt.Errorf("get document: %w", err)
			}
			if doc == nil {
				return fmt.Errorf("document not found: %s", docID)
			}

			// Build context
			var context strings.Builder
			context.WriteString(fmt.Sprintf("Title: %s\n", doc.Title))
			if doc.Abstract != "" {
				context.WriteString(fmt.Sprintf("Abstract: %s\n", doc.Abstract))
			}
			if doc.FullText != "" {
				text := doc.FullText
				if len(text) > 6000 {
					text = text[:6000] + "... (truncated)"
				}
				context.WriteString(fmt.Sprintf("\nFull Text:\n%s\n", text))
			}

			// Use arc-ai ask with input
			aiCmd := exec.Command("arc-ai", "ask", question)
			aiCmd.Stdin = strings.NewReader(context.String())
			var out bytes.Buffer
			aiCmd.Stdout = &out
			aiCmd.Stderr = &out
			if err := aiCmd.Run(); err != nil {
				return fmt.Errorf("arc-ai failed: %w\nOutput: %s", err, out.String())
			}

			fmt.Println("=== AI Answer ===")
			fmt.Println(out.String())
			fmt.Println()
			return nil
		},
	}

	return cmd
}

func newAIFlashcardsCmd(store library.LibraryStore) *cobra.Command {
	var (
		count    int
		storeRes bool
		tags     []string
	)

	cmd := &cobra.Command{
		Use:   "flashcards <document-id>",
		Short: "Generate flashcards from a document using AI",
		Long:  "Automatically generate Q&A flashcards from document content.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			docID := args[0]
			doc, err := store.GetDocument(docID)
			if err != nil {
				return fmt.Errorf("get document: %w", err)
			}
			if doc == nil {
				return fmt.Errorf("document not found: %s", docID)
			}

			// Build context
			var context strings.Builder
			context.WriteString(fmt.Sprintf("Title: %s\n", doc.Title))
			if len(doc.Authors) > 0 {
				context.WriteString(fmt.Sprintf("Authors: %s\n", strings.Join(doc.Authors, ", ")))
			}
			if doc.Abstract != "" {
				context.WriteString(fmt.Sprintf("Abstract: %s\n", doc.Abstract))
			}
			if doc.FullText != "" {
				text := doc.FullText
				if len(text) > 8000 {
					text = text[:8000] + "... (truncated)"
				}
				context.WriteString(fmt.Sprintf("\nFull Text:\n%s\n", text))
			}

			prompt := fmt.Sprintf(`Generate %d flashcards from this document in the following format:

Q: [question]
A: [answer]

Q: [question]
A: [answer]

Make the cards concise and focused on key concepts, definitions, and findings.`, count)

			aiCmd := exec.Command("arc-ai", "ask", prompt)
			aiCmd.Stdin = strings.NewReader(context.String())
			var out bytes.Buffer
			aiCmd.Stdout = &out
			aiCmd.Stderr = &out
			if err := aiCmd.Run(); err != nil {
				return fmt.Errorf("arc-ai failed: %w\nOutput: %s", err, out.String())
			}

			output := out.String()
			fmt.Println("=== Generated Flashcards ===")
			fmt.Println(output)
			fmt.Println()

			// Parse and store if requested
			if storeRes {
				cards := parseGeneratedFlashcards(output, docID, tags)
				for _, card := range cards {
					if err := store.AddFlashcard(card); err != nil {
						log.Printf("Failed to add card: %v", err)
					}
				}
				fmt.Printf("Added %d flashcards to library\n", len(cards))
			}

			return nil
		},
	}

	cmd.Flags().IntVarP(&count, "count", "n", 5, "Number of flashcards to generate")
	cmd.Flags().BoolVarP(&storeRes, "store", "s", false, "Store generated flashcards")
	cmd.Flags().StringSliceVarP(&tags, "tag", "t", nil, "Tags for generated cards")

	return cmd
}

func parseGeneratedFlashcards(text, docID string, tags []string) []*library.Flashcard {
	var cards []*library.Flashcard
	lines := strings.Split(text, "\n")

	var currentQ, currentA string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Q:") {
			currentQ = strings.TrimSpace(strings.TrimPrefix(line, "Q:"))
		} else if strings.HasPrefix(line, "A:") {
			currentA = strings.TrimSpace(strings.TrimPrefix(line, "A:"))
			if currentQ != "" && currentA != "" {
				cards = append(cards, &library.Flashcard{
					DocumentID: docID,
					Type:       "basic",
					Front:      currentQ,
					Back:       currentA,
					Tags:       tags,
					DueAt:      time.Now().AddDate(0, 0, 1),
					Interval:   0,
					Ease:       2.5,
				})
				currentQ, currentA = "", ""
			}
		}
	}

	return cards
}
