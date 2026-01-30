// Copyright (c) 2025 Arc Engineering
// SPDX-License-Identifier: MIT

package library

import (
	"time"
)

// Paper represents a paper in the library with its metadata and user data.
type Paper struct {
	ID        string    `json:"id" yaml:"id"`
	Path      string    `json:"path" yaml:"path"`
	Source    string    `json:"source" yaml:"source"` // arxiv, local, url
	SourceID  string    `json:"source_id,omitempty" yaml:"source_id,omitempty"`
	Title     string    `json:"title" yaml:"title"`
	Authors   []string  `json:"authors,omitempty" yaml:"authors,omitempty"`
	Abstract  string    `json:"abstract,omitempty" yaml:"abstract,omitempty"`
	Tags      []string  `json:"tags,omitempty" yaml:"tags,omitempty"`
	Notes     string    `json:"notes,omitempty" yaml:"notes,omitempty"`
	Rating    int       `json:"rating,omitempty" yaml:"rating,omitempty"` // 1-5
	ReadAt    time.Time `json:"read_at,omitempty" yaml:"read_at,omitempty"`
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	UpdatedAt time.Time `json:"updated_at" yaml:"updated_at"`
}

// Collection represents a named group of papers.
type Collection struct {
	ID          string    `json:"id" yaml:"id"`
	Name        string    `json:"name" yaml:"name"`
	Description string    `json:"description,omitempty" yaml:"description,omitempty"`
	PaperIDs    []string  `json:"paper_ids" yaml:"paper_ids"`
	CreatedAt   time.Time `json:"created_at" yaml:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" yaml:"updated_at"`
}

// Annotation represents a highlight or note on a specific part of a paper.
type Annotation struct {
	ID        string    `json:"id" yaml:"id"`
	PaperID   string    `json:"paper_id" yaml:"paper_id"`
	Type      string    `json:"type" yaml:"type"` // highlight, note, bookmark
	Content   string    `json:"content,omitempty" yaml:"content,omitempty"`
	Page      int       `json:"page,omitempty" yaml:"page,omitempty"`
	Position  string    `json:"position,omitempty" yaml:"position,omitempty"` // JSON coordinates
	Color     string    `json:"color,omitempty" yaml:"color,omitempty"`
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
}

// ReadingStatus represents the reading progress of a paper.
type ReadingStatus string

const (
	StatusUnread    ReadingStatus = "unread"
	StatusReading   ReadingStatus = "reading"
	StatusCompleted ReadingStatus = "completed"
	StatusArchived  ReadingStatus = "archived"
)
