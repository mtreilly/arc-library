// Copyright (c) 2025 Arc Engineering
// SPDX-License-Identifier: MIT

package library

import (
	"time"
)

// DocumentType represents the kind of document.
type DocumentType string

const (
	DocTypePaper      DocumentType = "paper"      // arXiv, conference, journal
	DocTypeBook       DocumentType = "book"       // Textbook, monograph
	DocTypeArticle    DocumentType = "article"    // Web article, blog post
	DocTypeVideo      DocumentType = "video"      // Lecture, tutorial
	DocTypeNote       DocumentType = "note"       // User-created note
	DocTypeRepo       DocumentType = "repo"       // Git repository
	DocTypeOther      DocumentType = "other"      // Miscellaneous
)

// Document represents any item in the library.
// It generalizes the previous "Paper" concept to support multiple content types.
type Document struct {
	ID          string         `json:"id" yaml:"id"`
	Type        DocumentType   `json:"type" yaml:"type"`
	Path        string         `json:"path" yaml:"path"`           // Local file or directory
	Source      string         `json:"source" yaml:"source"`       // "arxiv", "local", "url", "doi", etc.
	SourceID    string         `json:"source_id,omitempty" yaml:"source_id,omitempty"` // e.g., arXiv ID, DOI
	Title       string         `json:"title" yaml:"title"`
	Authors     []string       `json:"authors,omitempty" yaml:"authors,omitempty"`
	Abstract    string         `json:"abstract,omitempty" yaml:"abstract,omitempty"`
	FullText    string         `json:"full_text,omitempty" yaml:"full_text,omitempty"` // Extracted text (optional)
	Tags        []string       `json:"tags,omitempty" yaml:"tags,omitempty"`
	Notes       string         `json:"notes,omitempty" yaml:"notes,omitempty"`
	Rating      int            `json:"rating,omitempty" yaml:"rating,omitempty"` // 1-5
	ReadAt      time.Time      `json:"read_at,omitempty" yaml:"read_at,omitempty"`
	Status      ReadingStatus  `json:"status,omitempty" yaml:"status,omitempty"`
	CreatedAt   time.Time      `json:"created_at" yaml:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at" yaml:"updated_at"`

	// Type-specific metadata (unstructured)
	Meta        JSONMap        `json:"meta,omitempty" yaml:"meta,omitempty"`
}

// JSONMap is a flexible map for type-specific metadata.
type JSONMap map[string]any

// Collection represents a named group of documents.
type Collection struct {
	ID          string    `json:"id" yaml:"id"`
	Name        string    `json:"name" yaml:"name"`
	Description string    `json:"description,omitempty" yaml:"description,omitempty"`
	DocumentIDs []string  `json:"document_ids" yaml:"document_ids"` // Renamed from PaperIDs
	CreatedAt   time.Time `json:"created_at" yaml:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" yaml:"updated_at"`
}

// Annotation represents a highlight or note on a specific part of a document.
type Annotation struct {
	ID        string    `json:"id" yaml:"id"`
	DocumentID string   `json:"document_id" yaml:"document_id"` // Renamed from PaperID
	Type      string    `json:"type" yaml:"type"`               // highlight, note, bookmark
	Content   string    `json:"content,omitempty" yaml:"content,omitempty"`
	Page      int       `json:"page,omitempty" yaml:"page,omitempty"`
	Position  string    `json:"position,omitempty" yaml:"position,omitempty"` // JSON coordinates
	Color     string    `json:"color,omitempty" yaml:"color,omitempty"`
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
}

// ReadingSession tracks time spent reading a document.
type ReadingSession struct {
	ID        string    `json:"id" yaml:"id"`
	DocumentID string   `json:"document_id" yaml:"document_id"`
	StartAt   time.Time `json:"start_at" yaml:"start_at"`
	EndAt     time.Time `json:"end_at,omitempty" yaml:"end_at,omitempty"`
	PagesRead int       `json:"pages_read,omitempty" yaml:"pages_read,omitempty"`
	Notes     string    `json:"notes,omitempty" yaml:"notes,omitempty"`
}

// ReadingStatus represents the reading progress of a document.
type ReadingStatus string

const (
	StatusUnread    ReadingStatus = "unread"
	StatusReading   ReadingStatus = "reading"
	StatusCompleted ReadingStatus = "completed"
	StatusArchived  ReadingStatus = "archived"
)

// Flashcard represents a spaced repetition card.
type Flashcard struct {
	ID          string    `json:"id" yaml:"id"`
	DocumentID  string    `json:"document_id" yaml:"document_id"`
	Type        string    `json:"type" yaml:"type"` // "basic", "cloze"
	Front       string    `json:"front" yaml:"front"`
	Back        string    `json:"back,omitempty" yaml:"back,omitempty"`
	Cloze       string    `json:"cloze,omitempty" yaml:"cloze,omitempty"` // Cloze deletion pattern: {{c1::text}}
	Tags        []string  `json:"tags,omitempty" yaml:"tags,omitempty"`
	DueAt       time.Time `json:"due_at" yaml:"due_at"`
	Interval    int       `json:"interval" yaml:"interval"`     // days until next review
	Ease        float64   `json:"ease" yaml:"ease"`             // SM-2 ease factor (1.3-2.5)
	LastReview  time.Time `json:"last_review,omitempty" yaml:"last_review,omitempty"`
	CreatedAt   time.Time `json:"created_at" yaml:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" yaml:"updated_at"`
}

// FlashcardReview represents a single review attempt.
type FlashcardReview struct {
	ID          string    `json:"id" yaml:"id"`
	FlashcardID string    `json:"flashcard_id" yaml:"flashcard_id"`
	Quality     int       `json:"quality" yaml:"quality"` // 0-5: SM-2 quality score
	ReviewedAt  time.Time `json:"reviewed_at" yaml:"reviewed_at"`
	PrevInterval int      `json:"prev_interval,omitempty" yaml:"prev_interval,omitempty"`
	PrevEase    float64  `json:"prev_ease,omitempty" yaml:"prev_ease,omitempty"`
}

// ListOptions filters document listing.
type ListOptions struct {
	Tag    string
	Source string
	Search string
	Type   string
	Limit  int
}

// FlashcardListOptions filters flashcard listing.
type FlashcardListOptions struct {
	DocumentID string
	Tag        string
	Due        bool      // only due cards
	Limit      int
}
