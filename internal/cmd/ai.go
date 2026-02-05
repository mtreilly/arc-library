package cmd

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

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
