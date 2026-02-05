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

// Store provides persistence for library data using SQL.
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
	// In Phase 2, we'll add FTS5 table. For now, keep original schema but rename columns
	schema := `
	CREATE TABLE IF NOT EXISTS documents (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL DEFAULT 'paper',
		path TEXT NOT NULL,
		source TEXT NOT NULL,
		source_id TEXT,
		title TEXT NOT NULL,
		authors TEXT,
		abstract TEXT,
		full_text TEXT,
		tags TEXT,
		notes TEXT,
		rating INTEGER DEFAULT 0,
		status TEXT DEFAULT 'unread',
		read_at DATETIME,
		meta TEXT,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS collections (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		description TEXT,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS collection_documents (
		collection_id TEXT NOT NULL,
		document_id TEXT NOT NULL,
		added_at DATETIME NOT NULL,
		PRIMARY KEY (collection_id, document_id),
		FOREIGN KEY (collection_id) REFERENCES collections(id) ON DELETE CASCADE,
		FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS annotations (
		id TEXT PRIMARY KEY,
		document_id TEXT NOT NULL,
		type TEXT NOT NULL,
		content TEXT,
		page INTEGER,
		position TEXT,
		color TEXT,
		created_at DATETIME NOT NULL,
		FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS reading_sessions (
		id TEXT PRIMARY KEY,
		document_id TEXT NOT NULL,
		start_at DATETIME NOT NULL,
		end_at DATETIME,
		pages_read INTEGER,
		notes TEXT,
		FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_documents_source ON documents(source, source_id);
	CREATE INDEX IF NOT EXISTS idx_documents_tags ON documents(tags);
	CREATE INDEX IF NOT EXISTS idx_annotations_document ON annotations(document_id);
	CREATE INDEX IF NOT EXISTS idx_sessions_document ON reading_sessions(document_id);
	`;

	// Full-text search virtual table (FTS5)
	ftsSchema := `
	CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(
		doc_id UNINDEXED,
		title,
		abstract,
		full_text,
		tags,
		notes,
		content='',
		tokenize='porter'
	);

	-- Triggers to maintain FTS index
	CREATE TRIGGER IF NOT EXISTS documents_ai AFTER INSERT ON documents BEGIN
		INSERT INTO documents_fts (doc_id, title, abstract, full_text, tags, notes)
		VALUES (new.id, new.title, new.abstract, new.full_text, new.tags, new.notes);
	END;

	CREATE TRIGGER IF NOT EXISTS documents_ad AFTER DELETE ON documents BEGIN
		DELETE FROM documents_fts WHERE doc_id = old.id;
	END;

	CREATE TRIGGER IF NOT EXISTS documents_au AFTER UPDATE ON documents BEGIN
		UPDATE documents_fts SET
			title = new.title,
			abstract = new.abstract,
			full_text = new.full_text,
			tags = new.tags,
			notes = new.notes
		WHERE doc_id = old.id;
	END;
	`

	// Execute both schema batches
	_, err := s.db.Exec(schema)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ftsSchema)
	return err
}

// AddDocument adds a document to the library.
func (s *Store) AddDocument(doc *Document) error {
	if doc.ID == "" {
		doc.ID = uuid.New().String()
	}
	now := time.Now()
	doc.CreatedAt = now
	doc.UpdatedAt = now

	authorsJSON, _ := json.Marshal(doc.Authors)
	tagsJSON, _ := json.Marshal(doc.Tags)
	metaJSON, _ := json.Marshal(doc.Meta)

	_, err := s.db.Exec(`
		INSERT INTO documents (id, type, path, source, source_id, title, authors, abstract, full_text, tags, notes, rating, status, read_at, meta, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, doc.ID, doc.Type, doc.Path, doc.Source, doc.SourceID, doc.Title, string(authorsJSON), doc.Abstract, doc.FullText, string(tagsJSON), doc.Notes, doc.Rating, doc.Status, doc.ReadAt, string(metaJSON), doc.CreatedAt, doc.UpdatedAt)

	return err
}

// GetDocument retrieves a document by ID.
func (s *Store) GetDocument(id string) (*Document, error) {
	row := s.db.QueryRow(`
		SELECT id, type, path, source, source_id, title, authors, abstract, full_text, tags, notes, rating, status, read_at, meta, created_at, updated_at
		FROM documents WHERE id = ?
	`, id)
	return scanDocument(row)
}

// GetDocumentByPath retrieves a document by its filesystem path.
func (s *Store) GetDocumentByPath(path string) (*Document, error) {
	row := s.db.QueryRow(`
		SELECT id, type, path, source, source_id, title, authors, abstract, full_text, tags, notes, rating, status, read_at, meta, created_at, updated_at
		FROM documents WHERE path = ?
	`, path)
	return scanDocument(row)
}

// GetDocumentBySourceID retrieves a document by source and source ID (e.g., arxiv + 2304.00067).
func (s *Store) GetDocumentBySourceID(source, sourceID string) (*Document, error) {
	row := s.db.QueryRow(`
		SELECT id, type, path, source, source_id, title, authors, abstract, full_text, tags, notes, rating, status, read_at, meta, created_at, updated_at
		FROM documents WHERE source = ? AND source_id = ?
	`, source, sourceID)
	return scanDocument(row)
}

func scanDocument(row *sql.Row) (*Document, error) {
	var d Document
	var authorsJSON, tagsJSON, metaJSON string
	var sourceID, abstract, fullText, notes sql.NullString
	var status sql.NullString
	var readAt sql.NullTime

	err := row.Scan(&d.ID, &d.Type, &d.Path, &d.Source, &sourceID, &d.Title, &authorsJSON, &abstract, &fullText, &tagsJSON, &notes, &d.Rating, &status, &readAt, &metaJSON, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if sourceID.Valid {
		d.SourceID = sourceID.String
	}
	if abstract.Valid {
		d.Abstract = abstract.String
	}
	if fullText.Valid {
		d.FullText = fullText.String
	}
	if notes.Valid {
		d.Notes = notes.String
	}
	if status.Valid {
		d.Status = ReadingStatus(status.String)
	}
	if readAt.Valid {
		d.ReadAt = readAt.Time
	}

	json.Unmarshal([]byte(authorsJSON), &d.Authors)
	json.Unmarshal([]byte(tagsJSON), &d.Tags)
	json.Unmarshal([]byte(metaJSON), &d.Meta)

	return &d, nil
}

// ListDocuments returns all documents, optionally filtered.
func (s *Store) ListDocuments(opts *ListOptions) ([]*Document, error) {
	query := `SELECT id, type, path, source, source_id, title, authors, abstract, full_text, tags, notes, rating, status, read_at, meta, created_at, updated_at FROM documents WHERE 1=1`
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
			query += ` AND (title LIKE ? OR abstract LIKE ? OR notes LIKE ? OR full_text LIKE ?)`
			search := "%" + opts.Search + "%"
			args = append(args, search, search, search, search)
		}
		if opts.Type != "" {
			query += ` AND type = ?`
			args = append(args, opts.Type)
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

	var docs []*Document
	for rows.Next() {
		var d Document
		var authorsJSON, tagsJSON, metaJSON string
		var sourceID, abstract, fullText, notes sql.NullString
		var status sql.NullString
		var readAt sql.NullTime

		err := rows.Scan(&d.ID, &d.Type, &d.Path, &d.Source, &sourceID, &d.Title, &authorsJSON, &abstract, &fullText, &tagsJSON, &notes, &d.Rating, &status, &readAt, &metaJSON, &d.CreatedAt, &d.UpdatedAt)
		if err != nil {
			return nil, err
		}

		if sourceID.Valid {
			d.SourceID = sourceID.String
		}
		if abstract.Valid {
			d.Abstract = abstract.String
		}
		if fullText.Valid {
			d.FullText = fullText.String
		}
		if notes.Valid {
			d.Notes = notes.String
		}
		if status.Valid {
			d.Status = ReadingStatus(status.String)
		}
		if readAt.Valid {
			d.ReadAt = readAt.Time
		}

		json.Unmarshal([]byte(authorsJSON), &d.Authors)
		json.Unmarshal([]byte(tagsJSON), &d.Tags)
		json.Unmarshal([]byte(metaJSON), &d.Meta)

		docs = append(docs, &d)
	}

	return docs, nil
}

// UpdateDocument updates a document's metadata.
func (s *Store) UpdateDocument(doc *Document) error {
	doc.UpdatedAt = time.Now()

	authorsJSON, _ := json.Marshal(doc.Authors)
	tagsJSON, _ := json.Marshal(doc.Tags)
	metaJSON, _ := json.Marshal(doc.Meta)

	_, err := s.db.Exec(`
		UPDATE documents
		SET type = ?, title = ?, authors = ?, abstract = ?, full_text = ?, tags = ?, notes = ?, rating = ?, status = ?, read_at = ?, meta = ?, updated_at = ?
		WHERE id = ?
	`, doc.Type, doc.Title, string(authorsJSON), doc.Abstract, doc.FullText, string(tagsJSON), doc.Notes, doc.Rating, doc.Status, doc.ReadAt, string(metaJSON), doc.UpdatedAt, doc.ID)

	return err
}

// DeleteDocument removes a document from the library.
func (s *Store) DeleteDocument(id string) error {
	_, err := s.db.Exec(`DELETE FROM documents WHERE id = ?`, id)
	return err
}

// Tag operations (now use DocumentID)

func (s *Store) AddTag(documentID, tag string) error {
	doc, err := s.GetDocument(documentID)
	if err != nil {
		return err
	}
	if doc == nil {
		return fmt.Errorf("document not found: %s", documentID)
	}

	// Check if tag already exists
	for _, t := range doc.Tags {
		if strings.EqualFold(t, tag) {
			return nil // Already tagged
		}
	}

	doc.Tags = append(doc.Tags, tag)
	return s.UpdateDocument(doc)
}

func (s *Store) RemoveTag(documentID, tag string) error {
	doc, err := s.GetDocument(documentID)
	if err != nil {
		return err
	}
	if doc == nil {
		return fmt.Errorf("document not found: %s", documentID)
	}

	newTags := make([]string, 0, len(doc.Tags))
	for _, t := range doc.Tags {
		if !strings.EqualFold(t, tag) {
			newTags = append(newTags, t)
		}
	}

	doc.Tags = newTags
	return s.UpdateDocument(doc)
}

func (s *Store) ListTags() (map[string]int, error) {
	rows, err := s.db.Query(`SELECT tags FROM documents WHERE tags != '[]' AND tags != ''`)
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

// Collection operations (now use DocumentID)

func (s *Store) CreateCollection(name, description string) (*Collection, error) {
	c := &Collection{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	_, err := s.db.Exec(`
		INSERT INTO collections (id, name, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, c.ID, c.Name, c.Description, c.CreatedAt, c.UpdatedAt)

	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Store) GetCollection(idOrName string) (*Collection, error) {
	row := s.db.QueryRow(`
		SELECT id, name, description, created_at, updated_at
		FROM collections WHERE id = ? OR name = ?
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

	// Get document IDs
	rows, err := s.db.Query(`SELECT document_id FROM collection_documents WHERE collection_id = ?`, c.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var docID string
		if err := rows.Scan(&docID); err != nil {
			continue
		}
		c.DocumentIDs = append(c.DocumentIDs, docID)
	}

	return &c, nil
}

func (s *Store) ListCollections() ([]*Collection, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.name, c.description, c.created_at, c.updated_at, COUNT(cd.document_id) as doc_count
		FROM collections c
		LEFT JOIN collection_documents cd ON c.id = cd.collection_id
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
		var docCount int
		if err := rows.Scan(&c.ID, &c.Name, &desc, &c.CreatedAt, &c.UpdatedAt, &docCount); err != nil {
			continue
		}
		if desc.Valid {
			c.Description = desc.String
		}
		c.DocumentIDs = make([]string, docCount) // Just for count placeholder
		collections = append(collections, &c)
	}

	return collections, nil
}

func (s *Store) AddToCollection(collectionID, documentID string) error {
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO collection_documents (collection_id, document_id, added_at)
		VALUES (?, ?, ?)
	`, collectionID, documentID, time.Now())
	return err
}

func (s *Store) RemoveFromCollection(collectionID, documentID string) error {
	_, err := s.db.Exec(`DELETE FROM collection_documents WHERE collection_id = ? AND document_id = ?`, collectionID, documentID)
	return err
}

func (s *Store) DeleteCollection(id string) error {
	_, err := s.db.Exec(`DELETE FROM collections WHERE id = ?`, id)
	return err
}

// Annotation operations (now use DocumentID)

func (s *Store) AddAnnotation(ann *Annotation) error {
	if ann.ID == "" {
		ann.ID = uuid.New().String()
	}
	ann.CreatedAt = time.Now()

	_, err := s.db.Exec(`
		INSERT INTO annotations (id, document_id, type, content, page, position, color, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, ann.ID, ann.DocumentID, ann.Type, ann.Content, ann.Page, ann.Position, ann.Color, ann.CreatedAt)

	return err
}

func (s *Store) GetAnnotations(documentID string) ([]*Annotation, error) {
	rows, err := s.db.Query(`
		SELECT id, document_id, type, content, page, position, color, created_at
		FROM annotations WHERE document_id = ? ORDER BY page, created_at
	`, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var annotations []*Annotation
	for rows.Next() {
		var a Annotation
		var content, position, color sql.NullString
		var page sql.NullInt64

		if err := rows.Scan(&a.ID, &a.DocumentID, &a.Type, &content, &page, &position, &color, &a.CreatedAt); err != nil {
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

func (s *Store) DeleteAnnotation(id string) error {
	_, err := s.db.Exec(`DELETE FROM annotations WHERE id = ?`, id)
	return err
}

// Reading session operations (Phase 1)

func (s *Store) StartSession(documentID string) (*ReadingSession, error) {
	session := &ReadingSession{
		ID:          fmt.Sprintf("session:%d", time.Now().UnixNano()),
		DocumentID:  documentID,
		StartAt:     time.Now(),
	}
	_, err := s.db.Exec(`
		INSERT INTO reading_sessions (id, document_id, start_at)
		VALUES (?, ?, ?)
	`, session.ID, session.DocumentID, session.StartAt)
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (s *Store) EndSession(sessionID string, pagesRead int, notes string) error {
	_, err := s.db.Exec(`
		UPDATE reading_sessions
		SET end_at = ?, pages_read = ?, notes = ?
		WHERE id = ?
	`, time.Now(), pagesRead, notes, sessionID)
	return err
}

func (s *Store) ListSessions(documentID string) ([]*ReadingSession, error) {
	rows, err := s.db.Query(`
		SELECT id, document_id, start_at, end_at, pages_read, notes
		FROM reading_sessions WHERE document_id = ? ORDER BY start_at DESC
	`, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*ReadingSession
	for rows.Next() {
		var s ReadingSession
		var endAt sql.NullTime
		if err := rows.Scan(&s.ID, &s.DocumentID, &s.StartAt, &endAt, &s.PagesRead, &s.Notes); err != nil {
			continue
		}
		if endAt.Valid {
			s.EndAt = endAt.Time
		}
		sessions = append(sessions, &s)
	}
	return sessions, nil
}
