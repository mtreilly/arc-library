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

	// Flashcards tables
	flashcardSchema := `
	CREATE TABLE IF NOT EXISTS flashcards (
		id TEXT PRIMARY KEY,
		document_id TEXT NOT NULL,
		type TEXT NOT NULL,
		front TEXT NOT NULL,
		back TEXT,
		cloze TEXT,
		tags TEXT,
		due_at DATETIME NOT NULL,
		interval INTEGER NOT NULL,
		ease REAL NOT NULL,
		last_review DATETIME,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS flashcard_reviews (
		id TEXT PRIMARY KEY,
		flashcard_id TEXT NOT NULL,
		quality INTEGER NOT NULL,
		reviewed_at DATETIME NOT NULL,
		prev_interval INTEGER,
		prev_ease REAL,
		FOREIGN KEY (flashcard_id) REFERENCES flashcards(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_flashcards_due ON flashcards(due_at);
	CREATE INDEX IF NOT EXISTS idx_flashcards_document ON flashcards(document_id);
	CREATE INDEX IF NOT EXISTS idx_reviews_flashcard ON flashcard_reviews(flashcard_id);

	CREATE TABLE IF NOT EXISTS tasks (
		id TEXT PRIMARY KEY,
		description TEXT NOT NULL,
		collection_id TEXT,
		status TEXT NOT NULL DEFAULT 'todo',
		priority TEXT DEFAULT 'medium',
		tags TEXT DEFAULT '[]',
		due_at DATETIME,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		FOREIGN KEY (collection_id) REFERENCES collections(id) ON DELETE SET NULL
	);

	CREATE INDEX IF NOT EXISTS idx_tasks_collection ON tasks(collection_id);
	CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
	CREATE INDEX IF NOT EXISTS idx_tasks_due ON tasks(due_at);

	CREATE TABLE IF NOT EXISTS saved_searches (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		query TEXT NOT NULL,
		tag TEXT,
		source TEXT,
		type TEXT,
		description TEXT,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_saved_searches_name ON saved_searches(name);
	`

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

	// Execute all schema batches
	_, err := s.db.Exec(schema)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(flashcardSchema)
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
	var (
		query string
		args  []any
	)

	if opts != nil && opts.Search != "" {
		// Use FTS5 for full-text search
		query = `
			SELECT d.id, d.type, d.path, d.source, d.source_id, d.title, d.authors, d.abstract, d.full_text, d.tags, d.notes, d.rating, d.status, d.read_at, d.meta, d.created_at, d.updated_at
			FROM documents d
			JOIN documents_fts fts ON d.id = fts.rowid
			WHERE documents_fts MATCH ?`
		args = append(args, opts.Search)
	} else {
		query = `SELECT id, type, path, source, source_id, title, authors, abstract, full_text, tags, notes, rating, status, read_at, meta, created_at, updated_at FROM documents WHERE 1=1`
	}

	if opts != nil {
		if opts.Tag != "" {
			query += ` AND tags LIKE ?`
			args = append(args, "%\""+opts.Tag+"\"%")
		}
		if opts.Source != "" {
			query += ` AND source = ?`
			args = append(args, opts.Source)
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

// Flashcard operations (Phase 2)

func (s *Store) AddFlashcard(card *Flashcard) error {
	if card.ID == "" {
		card.ID = uuid.New().String()
	}
	now := time.Now()
	card.CreatedAt = now
	card.UpdatedAt = now

	tagsJSON, _ := json.Marshal(card.Tags)

	_, err := s.db.Exec(`
		INSERT INTO flashcards (id, document_id, type, front, back, cloze, tags, due_at, interval, ease, last_review, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, card.ID, card.DocumentID, card.Type, card.Front, card.Back, card.Cloze, string(tagsJSON), card.DueAt, card.Interval, card.Ease, card.LastReview, card.CreatedAt, card.UpdatedAt)

	return err
}

func (s *Store) GetFlashcard(id string) (*Flashcard, error) {
	row := s.db.QueryRow(`
		SELECT id, document_id, type, front, back, cloze, tags, due_at, interval, ease, last_review, created_at, updated_at
		FROM flashcards WHERE id = ?
	`, id)
	return scanFlashcard(row)
}

func scanFlashcard(row *sql.Row) (*Flashcard, error) {
	var c Flashcard
	var tagsJSON string
	var lastReview sql.NullTime

	err := row.Scan(&c.ID, &c.DocumentID, &c.Type, &c.Front, &c.Back, &c.Cloze, &tagsJSON, &c.DueAt, &c.Interval, &c.Ease, &lastReview, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if lastReview.Valid {
		c.LastReview = lastReview.Time
	}

	json.Unmarshal([]byte(tagsJSON), &c.Tags)
	return &c, nil
}

func scanFlashcardFromRows(rows *sql.Rows) (*Flashcard, error) {
	var c Flashcard
	var tagsJSON string
	var lastReview sql.NullTime

	err := rows.Scan(&c.ID, &c.DocumentID, &c.Type, &c.Front, &c.Back, &c.Cloze, &tagsJSON, &c.DueAt, &c.Interval, &c.Ease, &lastReview, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}

	if lastReview.Valid {
		c.LastReview = lastReview.Time
	}

	json.Unmarshal([]byte(tagsJSON), &c.Tags)
	return &c, nil
}

func (s *Store) ListFlashcards(opts *FlashcardListOptions) ([]*Flashcard, error) {
	query := `SELECT id, document_id, type, front, back, cloze, tags, due_at, interval, ease, last_review, created_at, updated_at FROM flashcards WHERE 1=1`
	var args []any

	if opts != nil {
		if opts.DocumentID != "" {
			query += ` AND document_id = ?`
			args = append(args, opts.DocumentID)
		}
		if opts.Tag != "" {
			query += ` AND tags LIKE ?`
			args = append(args, "%"+opts.Tag+"%")
		}
		if opts.Due {
			query += ` AND due_at <= ?`
			args = append(args, time.Now())
		}
	}

	query += ` ORDER BY due_at ASC`

	if opts != nil && opts.Limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, opts.Limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cards []*Flashcard
	for rows.Next() {
		c, err := scanFlashcardFromRows(rows)
		if err != nil {
			continue
		}
		cards = append(cards, c)
	}

	return cards, nil
}

func (s *Store) UpdateFlashcard(card *Flashcard) error {
	card.UpdatedAt = time.Now()

	tagsJSON, _ := json.Marshal(card.Tags)

	_, err := s.db.Exec(`
		UPDATE flashcards
		SET document_id = ?, type = ?, front = ?, back = ?, cloze = ?, tags = ?, due_at = ?, interval = ?, ease = ?, last_review = ?, updated_at = ?
		WHERE id = ?
	`, card.DocumentID, card.Type, card.Front, card.Back, card.Cloze, string(tagsJSON), card.DueAt, card.Interval, card.Ease, card.LastReview, card.UpdatedAt, card.ID)

	return err
}

func (s *Store) DeleteFlashcard(id string) error {
	_, err := s.db.Exec(`DELETE FROM flashcards WHERE id = ?`, id)
	return err
}

// ReviewFlashcard processes a quality rating (0-5) using the SM-2 algorithm
// and updates the card's interval and ease. Returns the updated card.
func (s *Store) ReviewFlashcard(id string, quality int) (*Flashcard, error) {
	card, err := s.GetFlashcard(id)
	if err != nil {
		return nil, err
	}
	if card == nil {
		return nil, fmt.Errorf("flashcard not found: %s", id)
	}

	now := time.Now()

	// Capture previous values for review record
	prevInterval := card.Interval
	prevEase := card.Ease
	if prevEase == 0 {
		prevEase = 2.5 // initial default
	}

	// SM-2 algorithm
	// quality: 0-5 (0=complete blackout, 5=perfect)
	// ease: factor by which interval multiplies (start at 2.5, range 1.3-2.5)
	// interval: days until next review

	ease := prevEase

	// Update ease
	ease = ease + (0.1 - (float64(5-quality)*(0.08+float64(5-quality)*0.02)))
	if ease < 1.3 {
		ease = 1.3
	}
	if ease > 2.5 {
		ease = 2.5
	}

	// Calculate new interval
	var interval int
	if quality < 3 {
		// Fail: reset to 1 day
		interval = 1
	} else {
		// Graduating or higher
		if prevInterval == 0 {
			// First successful review: interval = 1
			interval = 1
		} else if prevInterval == 1 {
			// Second: interval = 6 days
			interval = 6
		} else {
			// Normal: interval = interval * ease
			interval = int(float64(prevInterval) * ease)
		}
	}

	// Update card
	card.Interval = interval
	card.Ease = ease
	card.DueAt = now.AddDate(0, 0, interval)
	card.LastReview = now
	card.UpdatedAt = now

	// Save updated card
	if err := s.UpdateFlashcard(card); err != nil {
		return nil, err
	}

	// Record the review
	review := &FlashcardReview{
		ID:            fmt.Sprintf("review:%d", time.Now().UnixNano()),
		FlashcardID:   id,
		Quality:       quality,
		ReviewedAt:   now,
		PrevInterval: prevInterval,
		PrevEase:     prevEase,
	}
	_, err = s.db.Exec(`
		INSERT INTO flashcard_reviews (id, flashcard_id, quality, reviewed_at, prev_interval, prev_ease)
		VALUES (?, ?, ?, ?, ?, ?)
	`, review.ID, review.FlashcardID, review.Quality, review.ReviewedAt, review.PrevInterval, review.PrevEase)
	if err != nil {
		// Log but don't fail the review
		fmt.Printf("Warning: could not store review: %v\n", err)
	}

	return card, nil
}

func (s *Store) ListFlashcardReviews(flashcardID string) ([]*FlashcardReview, error) {
	rows, err := s.db.Query(`
		SELECT id, flashcard_id, quality, reviewed_at, prev_interval, prev_ease
		FROM flashcard_reviews WHERE flashcard_id = ? ORDER BY reviewed_at DESC
	`, flashcardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reviews []*FlashcardReview
	for rows.Next() {
		var r FlashcardReview
		if err := rows.Scan(&r.ID, &r.FlashcardID, &r.Quality, &r.ReviewedAt, &r.PrevInterval, &r.PrevEase); err != nil {
			continue
		}
		reviews = append(reviews, &r)
	}
	return reviews, nil
}

func (s *Store) GetDueFlashcards(now time.Time) ([]*Flashcard, error) {
	rows, err := s.db.Query(`
		SELECT id, document_id, type, front, back, cloze, tags, due_at, interval, ease, last_review, created_at, updated_at
		FROM flashcards WHERE due_at <= ? ORDER BY due_at ASC
	`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cards []*Flashcard
	for rows.Next() {
		c, err := scanFlashcardFromRows(rows)
		if err != nil {
			continue
		}
		cards = append(cards, c)
	}
	return cards, nil
}

// Task operations (Phase 3)

func (s *Store) AddTask(t *Task) error {
	if t.ID == "" {
		t.ID = fmt.Sprintf("task:%d", time.Now().UnixNano())
	}
	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()

	tagsJSON, _ := json.Marshal(t.Tags)
	
	var dueAt interface{}
	if t.DueAt != nil {
		dueAt = *t.DueAt
	}

	_, err := s.db.Exec(`
		INSERT INTO tasks (id, description, collection_id, status, priority, tags, due_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, t.ID, t.Description, t.CollectionID, t.Status, t.Priority, string(tagsJSON), dueAt, t.CreatedAt, t.UpdatedAt)
	
	return err
}

func (s *Store) GetTask(id string) (*Task, error) {
	var t Task
	var tagsJSON string
	var dueAt sql.NullTime
	var completedAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT id, description, collection_id, status, priority, tags, due_at, created_at, updated_at
		FROM tasks WHERE id = ?
	`, id).Scan(&t.ID, &t.Description, &t.CollectionID, &t.Status, &t.Priority, &tagsJSON, &dueAt, &t.CreatedAt, &t.UpdatedAt)
	
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(tagsJSON), &t.Tags)
	if dueAt.Valid {
		t.DueAt = &dueAt.Time
	}
	if completedAt.Valid {
		t.CompletedAt = &completedAt.Time
	}
	
	return &t, nil
}

func (s *Store) ListTasks(opts *TaskListOptions) ([]*Task, error) {
	query := `SELECT id, description, collection_id, status, priority, tags, due_at, created_at, updated_at FROM tasks WHERE 1=1`
	var args []any

	if opts != nil {
		if opts.CollectionID != "" {
			query += ` AND collection_id = ?`
			args = append(args, opts.CollectionID)
		}
		if opts.Status != "" {
			query += ` AND status = ?`
			args = append(args, opts.Status)
		}
	}

	query += ` ORDER BY created_at DESC`

	if opts != nil && opts.Limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, opts.Limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		var t Task
		var tagsJSON string
		var dueAt sql.NullTime
		
		err := rows.Scan(&t.ID, &t.Description, &t.CollectionID, &t.Status, &t.Priority, &tagsJSON, &dueAt, &t.CreatedAt, &t.UpdatedAt)
		if err != nil {
			continue
		}
		
		json.Unmarshal([]byte(tagsJSON), &t.Tags)
		if dueAt.Valid {
			t.DueAt = &dueAt.Time
		}
		
		tasks = append(tasks, &t)
	}
	
	return tasks, nil
}

func (s *Store) UpdateTask(t *Task) error {
	t.UpdatedAt = time.Now()
	tagsJSON, _ := json.Marshal(t.Tags)
	
	var dueAt interface{}
	if t.DueAt != nil {
		dueAt = *t.DueAt
	}

	_, err := s.db.Exec(`
		UPDATE tasks SET description = ?, collection_id = ?, status = ?, priority = ?, tags = ?, due_at = ?, updated_at = ?
		WHERE id = ?
	`, t.Description, t.CollectionID, t.Status, t.Priority, string(tagsJSON), dueAt, t.UpdatedAt, t.ID)
	
	return err
}

func (s *Store) DeleteTask(id string) error {
	_, err := s.db.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	return err
}

// SavedSearch operations

func (s *Store) SaveSearch(ss *SavedSearch) error {
	if ss.ID == "" {
		ss.ID = fmt.Sprintf("search:%d", time.Now().UnixNano())
	}
	ss.CreatedAt = time.Now()
	ss.UpdatedAt = time.Now()

	_, err := s.db.Exec(`
		INSERT INTO saved_searches (id, name, query, tag, source, type, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			query = excluded.query,
			tag = excluded.tag,
			source = excluded.source,
			type = excluded.type,
			description = excluded.description,
			updated_at = excluded.updated_at
	`, ss.ID, ss.Name, ss.Query, ss.Tag, ss.Source, ss.Type, ss.Description, ss.CreatedAt, ss.UpdatedAt)

	return err
}

func (s *Store) GetSavedSearch(idOrName string) (*SavedSearch, error) {
	var ss SavedSearch
	// Try by ID first, then by name
	err := s.db.QueryRow(`
		SELECT id, name, query, tag, source, type, description, created_at, updated_at
		FROM saved_searches WHERE id = ?
	`, idOrName).Scan(&ss.ID, &ss.Name, &ss.Query, &ss.Tag, &ss.Source, &ss.Type, &ss.Description, &ss.CreatedAt, &ss.UpdatedAt)

	if err == sql.ErrNoRows {
		// Try by name
		err = s.db.QueryRow(`
			SELECT id, name, query, tag, source, type, description, created_at, updated_at
			FROM saved_searches WHERE name = ?
		`, idOrName).Scan(&ss.ID, &ss.Name, &ss.Query, &ss.Tag, &ss.Source, &ss.Type, &ss.Description, &ss.CreatedAt, &ss.UpdatedAt)
	}

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &ss, nil
}

func (s *Store) ListSavedSearches() ([]*SavedSearch, error) {
	rows, err := s.db.Query(`
		SELECT id, name, query, tag, source, type, description, created_at, updated_at
		FROM saved_searches ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var searches []*SavedSearch
	for rows.Next() {
		var ss SavedSearch
		err := rows.Scan(&ss.ID, &ss.Name, &ss.Query, &ss.Tag, &ss.Source, &ss.Type, &ss.Description, &ss.CreatedAt, &ss.UpdatedAt)
		if err != nil {
			continue
		}
		searches = append(searches, &ss)
	}

	return searches, nil
}

func (s *Store) DeleteSavedSearch(id string) error {
	_, err := s.db.Exec(`DELETE FROM saved_searches WHERE id = ?`, id)
	return err
}
