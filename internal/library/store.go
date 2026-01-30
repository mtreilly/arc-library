// Copyright (c) 2025 Arc Engineering
// SPDX-License-Identifier: MIT

package library

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Store provides persistence for library data.
type Store struct {
	db *sql.DB
}

// NewStore creates a new library store and initializes the schema.
func NewStore(db *sql.DB) (*Store, error) {
	s := &Store{db: db}
	if err := s.initSchema(); err != nil {
		return nil, fmt.Errorf("init schema: %w", err)
	}
	return s, nil
}

func (s *Store) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS library_papers (
		id TEXT PRIMARY KEY,
		path TEXT NOT NULL,
		source TEXT NOT NULL,
		source_id TEXT,
		title TEXT NOT NULL,
		authors TEXT,
		abstract TEXT,
		tags TEXT,
		notes TEXT,
		rating INTEGER DEFAULT 0,
		status TEXT DEFAULT 'unread',
		read_at DATETIME,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS library_collections (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		description TEXT,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS library_collection_papers (
		collection_id TEXT NOT NULL,
		paper_id TEXT NOT NULL,
		added_at DATETIME NOT NULL,
		PRIMARY KEY (collection_id, paper_id),
		FOREIGN KEY (collection_id) REFERENCES library_collections(id) ON DELETE CASCADE,
		FOREIGN KEY (paper_id) REFERENCES library_papers(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS library_annotations (
		id TEXT PRIMARY KEY,
		paper_id TEXT NOT NULL,
		type TEXT NOT NULL,
		content TEXT,
		page INTEGER,
		position TEXT,
		color TEXT,
		created_at DATETIME NOT NULL,
		FOREIGN KEY (paper_id) REFERENCES library_papers(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_papers_source ON library_papers(source, source_id);
	CREATE INDEX IF NOT EXISTS idx_papers_tags ON library_papers(tags);
	CREATE INDEX IF NOT EXISTS idx_annotations_paper ON library_annotations(paper_id);
	`

	_, err := s.db.Exec(schema)
	return err
}

// AddPaper adds a paper to the library.
func (s *Store) AddPaper(paper *Paper) error {
	if paper.ID == "" {
		paper.ID = uuid.New().String()
	}
	now := time.Now()
	paper.CreatedAt = now
	paper.UpdatedAt = now

	authorsJSON, _ := json.Marshal(paper.Authors)
	tagsJSON, _ := json.Marshal(paper.Tags)

	_, err := s.db.Exec(`
		INSERT INTO library_papers (id, path, source, source_id, title, authors, abstract, tags, notes, rating, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, paper.ID, paper.Path, paper.Source, paper.SourceID, paper.Title, string(authorsJSON), paper.Abstract, string(tagsJSON), paper.Notes, paper.Rating, paper.CreatedAt, paper.UpdatedAt)

	return err
}

// GetPaper retrieves a paper by ID.
func (s *Store) GetPaper(id string) (*Paper, error) {
	row := s.db.QueryRow(`
		SELECT id, path, source, source_id, title, authors, abstract, tags, notes, rating, read_at, created_at, updated_at
		FROM library_papers WHERE id = ?
	`, id)
	return scanPaper(row)
}

// GetPaperByPath retrieves a paper by its filesystem path.
func (s *Store) GetPaperByPath(path string) (*Paper, error) {
	row := s.db.QueryRow(`
		SELECT id, path, source, source_id, title, authors, abstract, tags, notes, rating, read_at, created_at, updated_at
		FROM library_papers WHERE path = ?
	`, path)
	return scanPaper(row)
}

// GetPaperBySourceID retrieves a paper by source and source ID (e.g., arxiv + 2304.00067).
func (s *Store) GetPaperBySourceID(source, sourceID string) (*Paper, error) {
	row := s.db.QueryRow(`
		SELECT id, path, source, source_id, title, authors, abstract, tags, notes, rating, read_at, created_at, updated_at
		FROM library_papers WHERE source = ? AND source_id = ?
	`, source, sourceID)
	return scanPaper(row)
}

func scanPaper(row *sql.Row) (*Paper, error) {
	var p Paper
	var authorsJSON, tagsJSON string
	var sourceID, abstract, notes sql.NullString
	var readAt sql.NullTime

	err := row.Scan(&p.ID, &p.Path, &p.Source, &sourceID, &p.Title, &authorsJSON, &abstract, &tagsJSON, &notes, &p.Rating, &readAt, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if sourceID.Valid {
		p.SourceID = sourceID.String
	}
	if abstract.Valid {
		p.Abstract = abstract.String
	}
	if notes.Valid {
		p.Notes = notes.String
	}
	if readAt.Valid {
		p.ReadAt = readAt.Time
	}

	json.Unmarshal([]byte(authorsJSON), &p.Authors)
	json.Unmarshal([]byte(tagsJSON), &p.Tags)

	return &p, nil
}

// ListPapers returns all papers, optionally filtered.
func (s *Store) ListPapers(opts *ListOptions) ([]*Paper, error) {
	query := `SELECT id, path, source, source_id, title, authors, abstract, tags, notes, rating, read_at, created_at, updated_at FROM library_papers WHERE 1=1`
	var args []any

	if opts != nil {
		if opts.Tag != "" {
			query += ` AND tags LIKE ?`
			args = append(args, "%\""+opts.Tag+"\"%")
		}
		if opts.Source != "" {
			query += ` AND source = ?`
			args = append(args, opts.Source)
		}
		if opts.Search != "" {
			query += ` AND (title LIKE ? OR abstract LIKE ? OR notes LIKE ?)`
			search := "%" + opts.Search + "%"
			args = append(args, search, search, search)
		}
	}

	query += ` ORDER BY updated_at DESC`

	if opts != nil && opts.Limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, opts.Limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var papers []*Paper
	for rows.Next() {
		var p Paper
		var authorsJSON, tagsJSON string
		var sourceID, abstract, notes sql.NullString
		var readAt sql.NullTime

		err := rows.Scan(&p.ID, &p.Path, &p.Source, &sourceID, &p.Title, &authorsJSON, &abstract, &tagsJSON, &notes, &p.Rating, &readAt, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, err
		}

		if sourceID.Valid {
			p.SourceID = sourceID.String
		}
		if abstract.Valid {
			p.Abstract = abstract.String
		}
		if notes.Valid {
			p.Notes = notes.String
		}
		if readAt.Valid {
			p.ReadAt = readAt.Time
		}

		json.Unmarshal([]byte(authorsJSON), &p.Authors)
		json.Unmarshal([]byte(tagsJSON), &p.Tags)

		papers = append(papers, &p)
	}

	return papers, nil
}

// ListOptions filters paper listing.
type ListOptions struct {
	Tag    string
	Source string
	Search string
	Limit  int
}

// UpdatePaper updates a paper's metadata.
func (s *Store) UpdatePaper(paper *Paper) error {
	paper.UpdatedAt = time.Now()

	authorsJSON, _ := json.Marshal(paper.Authors)
	tagsJSON, _ := json.Marshal(paper.Tags)

	_, err := s.db.Exec(`
		UPDATE library_papers
		SET title = ?, authors = ?, abstract = ?, tags = ?, notes = ?, rating = ?, read_at = ?, updated_at = ?
		WHERE id = ?
	`, paper.Title, string(authorsJSON), paper.Abstract, string(tagsJSON), paper.Notes, paper.Rating, paper.ReadAt, paper.UpdatedAt, paper.ID)

	return err
}

// DeletePaper removes a paper from the library.
func (s *Store) DeletePaper(id string) error {
	_, err := s.db.Exec(`DELETE FROM library_papers WHERE id = ?`, id)
	return err
}

// AddTag adds a tag to a paper.
func (s *Store) AddTag(paperID, tag string) error {
	paper, err := s.GetPaper(paperID)
	if err != nil {
		return err
	}
	if paper == nil {
		return fmt.Errorf("paper not found: %s", paperID)
	}

	// Check if tag already exists
	for _, t := range paper.Tags {
		if strings.EqualFold(t, tag) {
			return nil // Already tagged
		}
	}

	paper.Tags = append(paper.Tags, tag)
	return s.UpdatePaper(paper)
}

// RemoveTag removes a tag from a paper.
func (s *Store) RemoveTag(paperID, tag string) error {
	paper, err := s.GetPaper(paperID)
	if err != nil {
		return err
	}
	if paper == nil {
		return fmt.Errorf("paper not found: %s", paperID)
	}

	var newTags []string
	for _, t := range paper.Tags {
		if !strings.EqualFold(t, tag) {
			newTags = append(newTags, t)
		}
	}

	paper.Tags = newTags
	return s.UpdatePaper(paper)
}

// ListTags returns all unique tags in the library.
func (s *Store) ListTags() (map[string]int, error) {
	rows, err := s.db.Query(`SELECT tags FROM library_papers WHERE tags != '[]' AND tags != ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tagCounts := make(map[string]int)
	for rows.Next() {
		var tagsJSON string
		if err := rows.Scan(&tagsJSON); err != nil {
			continue
		}
		var tags []string
		json.Unmarshal([]byte(tagsJSON), &tags)
		for _, tag := range tags {
			tagCounts[tag]++
		}
	}

	return tagCounts, nil
}

// CreateCollection creates a new collection.
func (s *Store) CreateCollection(name, description string) (*Collection, error) {
	c := &Collection{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	_, err := s.db.Exec(`
		INSERT INTO library_collections (id, name, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, c.ID, c.Name, c.Description, c.CreatedAt, c.UpdatedAt)

	if err != nil {
		return nil, err
	}
	return c, nil
}

// GetCollection retrieves a collection by ID or name.
func (s *Store) GetCollection(idOrName string) (*Collection, error) {
	row := s.db.QueryRow(`
		SELECT id, name, description, created_at, updated_at
		FROM library_collections WHERE id = ? OR name = ?
	`, idOrName, idOrName)

	var c Collection
	var desc sql.NullString
	err := row.Scan(&c.ID, &c.Name, &desc, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if desc.Valid {
		c.Description = desc.String
	}

	// Get paper IDs
	rows, err := s.db.Query(`SELECT paper_id FROM library_collection_papers WHERE collection_id = ?`, c.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var paperID string
		if err := rows.Scan(&paperID); err != nil {
			continue
		}
		c.PaperIDs = append(c.PaperIDs, paperID)
	}

	return &c, nil
}

// ListCollections returns all collections.
func (s *Store) ListCollections() ([]*Collection, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.name, c.description, c.created_at, c.updated_at, COUNT(cp.paper_id) as paper_count
		FROM library_collections c
		LEFT JOIN library_collection_papers cp ON c.id = cp.collection_id
		GROUP BY c.id
		ORDER BY c.name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var collections []*Collection
	for rows.Next() {
		var c Collection
		var desc sql.NullString
		var paperCount int
		if err := rows.Scan(&c.ID, &c.Name, &desc, &c.CreatedAt, &c.UpdatedAt, &paperCount); err != nil {
			continue
		}
		if desc.Valid {
			c.Description = desc.String
		}
		c.PaperIDs = make([]string, paperCount) // Just for count
		collections = append(collections, &c)
	}

	return collections, nil
}

// AddToCollection adds a paper to a collection.
func (s *Store) AddToCollection(collectionID, paperID string) error {
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO library_collection_papers (collection_id, paper_id, added_at)
		VALUES (?, ?, ?)
	`, collectionID, paperID, time.Now())
	return err
}

// RemoveFromCollection removes a paper from a collection.
func (s *Store) RemoveFromCollection(collectionID, paperID string) error {
	_, err := s.db.Exec(`DELETE FROM library_collection_papers WHERE collection_id = ? AND paper_id = ?`, collectionID, paperID)
	return err
}

// DeleteCollection removes a collection.
func (s *Store) DeleteCollection(id string) error {
	_, err := s.db.Exec(`DELETE FROM library_collections WHERE id = ?`, id)
	return err
}

// AddAnnotation adds an annotation to a paper.
func (s *Store) AddAnnotation(ann *Annotation) error {
	if ann.ID == "" {
		ann.ID = uuid.New().String()
	}
	ann.CreatedAt = time.Now()

	_, err := s.db.Exec(`
		INSERT INTO library_annotations (id, paper_id, type, content, page, position, color, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, ann.ID, ann.PaperID, ann.Type, ann.Content, ann.Page, ann.Position, ann.Color, ann.CreatedAt)

	return err
}

// GetAnnotations retrieves all annotations for a paper.
func (s *Store) GetAnnotations(paperID string) ([]*Annotation, error) {
	rows, err := s.db.Query(`
		SELECT id, paper_id, type, content, page, position, color, created_at
		FROM library_annotations WHERE paper_id = ? ORDER BY page, created_at
	`, paperID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var annotations []*Annotation
	for rows.Next() {
		var a Annotation
		var content, position, color sql.NullString
		var page sql.NullInt64

		if err := rows.Scan(&a.ID, &a.PaperID, &a.Type, &content, &page, &position, &color, &a.CreatedAt); err != nil {
			continue
		}

		if content.Valid {
			a.Content = content.String
		}
		if position.Valid {
			a.Position = position.String
		}
		if color.Valid {
			a.Color = color.String
		}
		if page.Valid {
			a.Page = int(page.Int64)
		}

		annotations = append(annotations, &a)
	}

	return annotations, nil
}

// DeleteAnnotation removes an annotation.
func (s *Store) DeleteAnnotation(id string) error {
	_, err := s.db.Exec(`DELETE FROM library_annotations WHERE id = ?`, id)
	return err
}
