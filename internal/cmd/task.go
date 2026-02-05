// Copyright (c) 2025 Arc Engineering
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"time"

	"github.com/mtreilly/arc-library/internal/library"
	"github.com/spf13/cobra"
	"github.com/yourorg/arc-sdk/config"
	"github.com/yourorg/arc-sdk/output"
)

func newTaskCmd(cfg *config.Config, store library.LibraryStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Manage tasks and projects",
		Long:  "Create and track tasks associated with document collections.",
	}

	cmd.AddCommand(newTaskAddCmd(store))
	cmd.AddCommand(newTaskListCmd(store))
	cmd.AddCommand(newTaskDoneCmd(store))
	cmd.AddCommand(newTaskDeleteCmd(store))

	return cmd
}

func newTaskAddCmd(store library.LibraryStore) *cobra.Command {
	var (
		collection string
		due        string
		priority   string
		tags       []string
	)

	cmd := &cobra.Command{
		Use:   "add <description>",
		Short: "Add a new task",
		Long:  "Create a task associated with a collection.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			description := ""
			if len(args) > 0 {
				description = args[0]
			}

			// Verify collection exists
			var collID string
			if collection != "" {
				coll, err := store.GetCollection(collection)
				if err != nil {
					return fmt.Errorf("get collection: %w", err)
				}
				if coll == nil {
					// Auto-create collection
					coll, err = store.CreateCollection(collection, "Auto-created from task")
					if err != nil {
						return fmt.Errorf("create collection: %w", err)
					}
					fmt.Printf("Created collection: %s\n", collection)
				}
				collID = coll.ID
			}

			task := &library.Task{
				Description:   description,
				CollectionID:  collID,
				Status:        "todo",
				Priority:      priority,
				Tags:          tags,
				CreatedAt:     time.Now(),
				UpdatedAt:     time.Now(),
			}

			if due != "" {
				dueTime, err := time.Parse("2006-01-02", due)
				if err != nil {
					return fmt.Errorf("invalid due date (use YYYY-MM-DD): %w", err)
				}
				task.DueAt = &dueTime
			}

			if err := store.AddTask(task); err != nil {
				return fmt.Errorf("add task: %w", err)
			}

			fmt.Printf("Task created: %s\n", task.ID)
			fmt.Printf("Description: %s\n", task.Description)
			if collection != "" {
				fmt.Printf("Collection: %s\n", collection)
			}
			if due != "" {
				fmt.Printf("Due: %s\n", due)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&collection, "collection", "c", "", "Associate with collection")
	cmd.Flags().StringVarP(&due, "due", "d", "", "Due date (YYYY-MM-DD)")
	cmd.Flags().StringVarP(&priority, "priority", "p", "medium", "Priority (low/medium/high)")
	cmd.Flags().StringSliceVarP(&tags, "tag", "t", nil, "Tags")

	return cmd
}

func newTaskListCmd(store library.LibraryStore) *cobra.Command {
	var (
		collection string
		status     string
		all        bool
		out        output.OutputOptions
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.Resolve(); err != nil {
				return err
			}

			opts := &library.TaskListOptions{}
			if collection != "" {
				coll, err := store.GetCollection(collection)
				if err != nil {
					return fmt.Errorf("get collection: %w", err)
				}
				if coll != nil {
					opts.CollectionID = coll.ID
				}
			}
			if status != "" {
				opts.Status = status
			}
			if !all {
				// Default: show only incomplete
				opts.Status = "todo"
			}

			tasks, err := store.ListTasks(opts)
			if err != nil {
				return fmt.Errorf("list tasks: %w", err)
			}

			if out.Is(output.OutputJSON) {
				return output.JSON(tasks)
			}

			if len(tasks) == 0 {
				fmt.Println("No tasks found.")
				return nil
			}

			// Group by status
			fmt.Printf("Tasks: %d\n\n", len(tasks))

			table := output.NewTable("ID", "Description", "Collection", "Due", "Priority")
			for _, t := range tasks {
				desc := truncate(t.Description, 40)
				collName := ""
				if t.CollectionID != "" {
					coll, _ := store.GetCollection(t.CollectionID)
					if coll != nil {
						collName = truncate(coll.Name, 15)
					}
				}
				dueStr := ""
				if t.DueAt != nil {
					dueStr = t.DueAt.Format("2006-01-02")
					if t.DueAt.Before(time.Now()) {
						dueStr += " (!)"
					}
				}
				table.AddRow(truncate(t.ID, 8), desc, collName, dueStr, t.Priority)
			}
			table.Render()

			return nil
		},
	}

	cmd.Flags().StringVarP(&collection, "collection", "c", "", "Filter by collection")
	cmd.Flags().StringVarP(&status, "status", "s", "", "Filter by status (todo/done)")
	cmd.Flags().BoolVarP(&all, "all", "a", false, "Show all tasks including completed")
	out.AddOutputFlags(cmd, output.OutputTable)

	return cmd
}

func newTaskDoneCmd(store library.LibraryStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "done <task-id>",
		Short: "Mark a task as complete",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]
			task, err := store.GetTask(taskID)
			if err != nil {
				return fmt.Errorf("get task: %w", err)
			}
			if task == nil {
				return fmt.Errorf("task not found: %s", taskID)
			}

			task.Status = "done"
			task.CompletedAt = &[]time.Time{time.Now()}[0]
			task.UpdatedAt = time.Now()

			if err := store.UpdateTask(task); err != nil {
				return fmt.Errorf("update task: %w", err)
			}

			fmt.Printf("Task completed: %s\n", task.Description)
			return nil
		},
	}

	return cmd
}

func newTaskDeleteCmd(store library.LibraryStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <task-id>",
		Short: "Delete a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]
			if err := store.DeleteTask(taskID); err != nil {
				return fmt.Errorf("delete task: %w", err)
			}
			fmt.Printf("Task deleted: %s\n", taskID)
			return nil
		},
	}

	return cmd
}
