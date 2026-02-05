// Copyright (c) 2025 Arc Engineering
// SPDX-License-Identifier: MIT

package library

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yourorg/arc-sdk/store"
)

// KVStore implements the LibraryStore interface using arc-sdk/store.KVStore.
type KVStore struct {
	kv store.KVStore
}

// NewKVStore creates a new library store backed by the given KVStore.
func NewKVStore(kv store.KVStore) (*KVStore, error) {
	s := &KVStore{kv: kv}
	return s, nil
}

// generateKey creates namespaced keys for different entity types.
func (s *KVStore) generateKey(prefix, id string) string {
	return fmt.Sprintf("arc-library:%s:%s", prefix, id)
}

// Document operations

func (s *KVStore) AddDocument(doc *Document) error {
	if doc.ID == "" {
		doc.ID = fmt.Sprintf("doc:%d", time.Now().UnixNano())
	}
	now := time.Now()
	doc.CreatedAt = now
	doc.UpdatedAt = now

	data, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal document: %w", err)
	}

	ctx := context.Background()
	key := s.generateKey("doc", doc.ID)
	if err := s.kv.Set(ctx, key, data); err != nil {
		return fmt.Errorf("set document: %w", err)
	}

	// Index by path for deduplication
	if doc.Path != "" {
		if err := s.kv.Set(ctx, s.generateKey("doc:path", doc.Path), []byte(doc.ID)); err != nil {
			// Log but don't fail - indices can be rebuilt
		}
	}

	// Index by source+source_id if present
	if doc.Source != "" && doc.SourceID != "" {
		sourceKey := fmt.Sprintf("%s:%s", doc.Source, doc.SourceID)
		if err := s.kv.Set(ctx, s.generateKey("doc:source", sourceKey), []byte(doc.ID)); err != nil {
			// Log but don't fail
		}
	}

	// Add to main document index
	if err := s.addToDocumentIndex(doc.ID); err != nil {
		// Log but don't fail
	}

	return nil
}

func (s *KVStore) GetDocument(id string) (*Document, error) {
	ctx := context.Background()
	key := s.generateKey("doc", id)
	data, err := s.kv.Get(ctx, key)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	var d Document
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("unmarshal document: %w", err)
	}
	return &d, nil
}

func (s *KVStore) GetDocumentByPath(path string) (*Document, error) {
	ctx := context.Background()
	key := s.generateKey("doc:path", path)
	idData, err := s.kv.Get(ctx, key)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return s.GetDocument(string(idData))
}

func (s *KVStore) GetDocumentBySourceID(source, sourceID string) (*Document, error) {
	ctx := context.Background()
	sourceKey := fmt.Sprintf("%s:%s", source, sourceID)
	key := s.generateKey("doc:source", sourceKey)
	idData, err := s.kv.Get(ctx, key)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return s.GetDocument(string(idData))
}

func (s *KVStore) ListDocuments(opts *ListOptions) ([]*Document, error) {
	ctx := context.Background()

	// Get all document IDs from the index
	indexKey := s.generateKey("index", "documents")
	idsData, err := s.kv.Get(ctx, indexKey)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}

	var ids []string
	if err == nil {
		if err := json.Unmarshal(idsData, &ids); err != nil {
			ids = []string{}
		}
	}

	var docs []*Document
	for _, id := range ids {
		doc, err := s.GetDocument(id)
		if err != nil {
			continue
		}
		if doc == nil {
			continue
		}

		// Apply filters
		if opts != nil {
			if opts.Tag != "" {
				found := false
				for _, t := range doc.Tags {
					if strings.EqualFold(t, opts.Tag) {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
			if opts.Source != "" && doc.Source != opts.Source {
				continue
			}
			if opts.Search != "" {
				search := strings.ToLower(opts.Search)
				title := strings.ToLower(doc.Title)
				abstract := strings.ToLower(doc.Abstract)
				notes := strings.ToLower(doc.Notes)
				fullText := strings.ToLower(doc.FullText)
				if !strings.Contains(title, search) && !strings.Contains(abstract, search) &&
					!strings.Contains(notes, search) && !strings.Contains(fullText, search) {
					continue
				}
			}
			if opts.Type != "" && doc.Type != DocumentType(opts.Type) {
				continue
			}
		}

		docs = append(docs, doc)

		if opts != nil && opts.Limit > 0 && len(docs) >= opts.Limit {
			break
		}
	}

	return docs, nil
}

func (s *KVStore) UpdateDocument(doc *Document) error {
	existing, err := s.GetDocument(doc.ID)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("document not found: %s", doc.ID)
	}

	doc.CreatedAt = existing.CreatedAt
	doc.UpdatedAt = time.Now()

	data, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal document: %w", err)
	}

	ctx := context.Background()
	key := s.generateKey("doc", doc.ID)
	if err := s.kv.Set(ctx, key, data); err != nil {
		return fmt.Errorf("set document: %w", err)
	}

	// Update path index if changed
	if existing.Path != doc.Path {
		_ = s.kv.Delete(ctx, s.generateKey("doc:path", existing.Path))
		if doc.Path != "" {
			_ = s.kv.Set(ctx, s.generateKey("doc:path", doc.Path), []byte(doc.ID))
		}
	}

	// Update source index if changed
	if existing.Source != doc.Source || existing.SourceID != doc.SourceID {
		if existing.Source != "" && existing.SourceID != "" {
			oldSourceKey := fmt.Sprintf("%s:%s", existing.Source, existing.SourceID)
			_ = s.kv.Delete(ctx, s.generateKey("doc:source", oldSourceKey))
		}
		if doc.Source != "" && doc.SourceID != "" {
			newSourceKey := fmt.Sprintf("%s:%s", doc.Source, doc.SourceID)
			_ = s.kv.Set(ctx, s.generateKey("doc:source", newSourceKey), []byte(doc.ID))
		}
	}

	return nil
}

func (s *KVStore) DeleteDocument(id string) error {
	doc, err := s.GetDocument(id)
	if err != nil {
		return err
	}
	if doc == nil {
		return nil // Already deleted
	}

	ctx := context.Background()

	// Remove from all collections
	collections, err := s.ListCollections()
	if err != nil {
		return err
	}
	for _, c := range collections {
		s.RemoveFromCollection(c.ID, id)
	}

	// Delete annotations
	anns, _ := s.GetAnnotations(id)
	for _, a := range anns {
		s.DeleteAnnotation(a.ID)
	}

	// Delete indices
	_ = s.kv.Delete(ctx, s.generateKey("doc:path", doc.Path))
	if doc.Source != "" && doc.SourceID != "" {
		sourceKey := fmt.Sprintf("%s:%s", doc.Source, doc.SourceID)
		_ = s.kv.Delete(ctx, s.generateKey("doc:source", sourceKey))
	}

	// Remove from document index
	if err := s.removeFromDocumentIndex(id); err != nil {
		// Log but continue
	}

	// Delete document data
	key := s.generateKey("doc", id)
	return s.kv.Delete(ctx, key)
}

// Document index maintenance

func (s *KVStore) addToDocumentIndex(docID string) error {
	ctx := context.Background()
	indexKey := s.generateKey("index", "documents")

	ids, err := s.getDocumentIndex()
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}

	// Avoid duplicates
	for _, id := range ids {
		if id == docID {
			return nil
		}
	}

	ids = append(ids, docID)
	data, _ := json.Marshal(ids)
	return s.kv.Set(ctx, indexKey, data)
}

func (s *KVStore) removeFromDocumentIndex(docID string) error {
	ctx := context.Background()
	indexKey := s.generateKey("index", "documents")

	ids, err := s.getDocumentIndex()
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return err
	}

	newIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != docID {
			newIDs = append(newIDs, id)
		}
	}

	data, _ := json.Marshal(newIDs)
	return s.kv.Set(ctx, indexKey, data)
}

func (s *KVStore) getDocumentIndex() ([]string, error) {
	ctx := context.Background()
	indexKey := s.generateKey("index", "documents")
	data, err := s.kv.Get(ctx, indexKey)
	if err != nil {
		return nil, err
	}
	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		return nil, fmt.Errorf("unmarshal document index: %w", err)
	}
	return ids, nil
}

// Tag operations (use DocumentID)

func (s *KVStore) AddTag(documentID, tag string) error {
	doc, err := s.GetDocument(documentID)
	if err != nil {
		return err
	}
	if doc == nil {
		return fmt.Errorf("document not found: %s", documentID)
	}

	// Check if already tagged
	for _, t := range doc.Tags {
		if strings.EqualFold(t, tag) {
			return nil
		}
	}

	doc.Tags = append(doc.Tags, tag)
	return s.UpdateDocument(doc)
}

func (s *KVStore) RemoveTag(documentID, tag string) error {
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

func (s *KVStore) ListTags() (map[string]int, error) {
	docs, err := s.ListDocuments(nil)
	if err != nil {
		return nil, err
	}

	tagCounts := make(map[string]int)
	for _, d := range docs {
		for _, tag := range d.Tags {
			tagCounts[tag]++
		}
	}
	return tagCounts, nil
}

// Collection operations

func (s *KVStore) CreateCollection(name, description string) (*Collection, error) {
	c := &Collection{
		ID:          fmt.Sprintf("collection:%d", time.Now().UnixNano()),
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	ctx := context.Background()
	key := s.generateKey("collection", c.ID)
	data, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("marshal collection: %w", err)
	}
	if err := s.kv.Set(ctx, key, data); err != nil {
		return nil, err
	}

	// Maintain collection index
	if err := s.addToCollectionIndex(c.ID); err != nil {
		// Log but don't fail
	}

	return c, nil
}

func (s *KVStore) GetCollection(idOrName string) (*Collection, error) {
	// Try by ID first
	c, err := s.getCollectionByID(idOrName)
	if err != nil {
		return nil, err
	}
	if c != nil {
		return c, nil
	}
	// Then search by name
	return s.getCollectionByName(idOrName)
}

func (s *KVStore) getCollectionByID(id string) (*Collection, error) {
	ctx := context.Background()
	key := s.generateKey("collection", id)
	data, err := s.kv.Get(ctx, key)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	var c Collection
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("unmarshal collection: %w", err)
	}
	return &c, nil
}

func (s *KVStore) getCollectionByName(name string) (*Collection, error) {
	collections, err := s.ListCollections()
	if err != nil {
		return nil, err
	}
	for _, c := range collections {
		if strings.EqualFold(c.Name, name) {
			return c, nil
		}
	}
	return nil, nil
}

func (s *KVStore) ListCollections() ([]*Collection, error) {
	ctx := context.Background()

	indexKey := s.generateKey("index", "collections")
	idsData, err := s.kv.Get(ctx, indexKey)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}

	var ids []string
	if err == nil {
		if err := json.Unmarshal(idsData, &ids); err != nil {
			ids = []string{}
		}
	}

	var collections []*Collection
	for _, id := range ids {
		c, err := s.getCollectionByID(id)
		if err != nil {
			continue
		}
		if c == nil {
			continue
		}
		collections = append(collections, c)
	}

	// Sort by name (simple insertion sort for small collections)
	for i := 1; i < len(collections); i++ {
		j := i
		for j > 0 && collections[j-1].Name > collections[j].Name {
			collections[j-1], collections[j] = collections[j], collections[j-1]
			j--
		}
	}

	return collections, nil
}

func (s *KVStore) AddToCollection(collectionID, documentID string) error {
	c, err := s.getCollectionByID(collectionID)
	if err != nil {
		return err
	}
	if c == nil {
		return fmt.Errorf("collection not found: %s", collectionID)
	}

	// Check if already in collection
	for _, did := range c.DocumentIDs {
		if did == documentID {
			return nil
		}
	}

	c.DocumentIDs = append(c.DocumentIDs, documentID)
	c.UpdatedAt = time.Now()

	ctx := context.Background()
	key := s.generateKey("collection", c.ID)
	data, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal collection: %w", err)
	}
	return s.kv.Set(ctx, key, data)
}

func (s *KVStore) RemoveFromCollection(collectionID, documentID string) error {
	c, err := s.getCollectionByID(collectionID)
	if err != nil {
		return err
	}
	if c == nil {
		return fmt.Errorf("collection not found: %s", collectionID)
	}

	newIDs := make([]string, 0, len(c.DocumentIDs))
	for _, did := range c.DocumentIDs {
		if did != documentID {
			newIDs = append(newIDs, did)
		}
	}

	c.DocumentIDs = newIDs
	c.UpdatedAt = time.Now()

	ctx := context.Background()
	key := s.generateKey("collection", c.ID)
	data, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal collection: %w", err)
	}
	return s.kv.Set(ctx, key, data)
}

func (s *KVStore) DeleteCollection(id string) error {
	c, err := s.getCollectionByID(id)
	if err != nil {
		return err
	}
	if c == nil {
		return nil
	}

	ctx := context.Background()

	// Delete collection data
	key := s.generateKey("collection", id)
	if err := s.kv.Delete(ctx, key); err != nil {
		return err
	}

	// Remove from index
	if err := s.removeFromCollectionIndex(id); err != nil {
		// Log but don't fail
	}

	return nil
}

func (s *KVStore) addToCollectionIndex(collectionID string) error {
	ctx := context.Background()
	indexKey := s.generateKey("index", "collections")
	ids, err := s.getCollectionIndex()
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	for _, id := range ids {
		if id == collectionID {
			return nil
		}
	}
	ids = append(ids, collectionID)
	data, _ := json.Marshal(ids)
	return s.kv.Set(ctx, indexKey, data)
}

func (s *KVStore) removeFromCollectionIndex(collectionID string) error {
	ctx := context.Background()
	indexKey := s.generateKey("index", "collections")
	ids, err := s.getCollectionIndex()
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return err
	}
	newIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != collectionID {
			newIDs = append(newIDs, id)
		}
	}
	data, _ := json.Marshal(newIDs)
	return s.kv.Set(ctx, indexKey, data)
}

func (s *KVStore) getCollectionIndex() ([]string, error) {
	ctx := context.Background()
	indexKey := s.generateKey("index", "collections")
	data, err := s.kv.Get(ctx, indexKey)
	if err != nil {
		return nil, err
	}
	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		return nil, fmt.Errorf("unmarshal collection index: %w", err)
	}
	return ids, nil
}

// Annotation operations (use DocumentID)

func (s *KVStore) AddAnnotation(ann *Annotation) error {
	if ann.ID == "" {
		ann.ID = fmt.Sprintf("annotation:%d", time.Now().UnixNano())
	}
	ann.CreatedAt = time.Now()

	ctx := context.Background()
	key := s.generateKey("annotation", ann.ID)
	data, err := json.Marshal(ann)
	if err != nil {
		return fmt.Errorf("marshal annotation: %w", err)
	}
	if err := s.kv.Set(ctx, key, data); err != nil {
		return err
	}

	// Add to document's annotation index
	if err := s.addToDocumentAnnotationsIndex(ann.DocumentID, ann.ID); err != nil {
		// Log but don't fail
	}

	return nil
}

func (s *KVStore) GetAnnotations(documentID string) ([]*Annotation, error) {
	ctx := context.Background()

	// Get annotation IDs for this document from the index
	indexKey := s.generateKey("index", "doc:annotations:"+documentID)
	idsData, err := s.kv.Get(ctx, indexKey)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}

	var ids []string
	if err == nil {
		if err := json.Unmarshal(idsData, &ids); err != nil {
			ids = []string{}
		}
	}

	var anns []*Annotation
	for _, id := range ids {
		key := s.generateKey("annotation", id)
		data, err := s.kv.Get(ctx, key)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				continue // Orphaned annotation - skip
			}
			return nil, err
		}
		var a Annotation
		if err := json.Unmarshal(data, &a); err != nil {
			continue
		}
		anns = append(anns, &a)
	}

	return anns, nil
}

func (s *KVStore) DeleteAnnotation(id string) error {
	ctx := context.Background()

	// Get annotation first to find its document
	key := s.generateKey("annotation", id)
	data, err := s.kv.Get(ctx, key)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return err
	}
	var a Annotation
	if err := json.Unmarshal(data, &a); err != nil {
		return fmt.Errorf("unmarshal annotation: %w", err)
	}

	// Remove from document's annotation index
	_ = s.removeFromDocumentAnnotationsIndex(a.DocumentID, id)

	// Delete annotation
	return s.kv.Delete(ctx, key)
}

func (s *KVStore) addToDocumentAnnotationsIndex(documentID, annotationID string) error {
	ctx := context.Background()
	indexKey := s.generateKey("index", "doc:annotations:"+documentID)
	ids, err := s.getDocumentAnnotationsIndex(documentID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	// Avoid duplicates
	for _, id := range ids {
		if id == annotationID {
			return nil
		}
	}
	ids = append(ids, annotationID)
	data, _ := json.Marshal(ids)
	return s.kv.Set(ctx, indexKey, data)
}

func (s *KVStore) removeFromDocumentAnnotationsIndex(documentID, annotationID string) error {
	ctx := context.Background()
	indexKey := s.generateKey("index", "doc:annotations:"+documentID)
	ids, err := s.getDocumentAnnotationsIndex(documentID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return err
	}
	newIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != annotationID {
			newIDs = append(newIDs, id)
		}
	}
	data, _ := json.Marshal(newIDs)
	return s.kv.Set(ctx, indexKey, data)
}

func (s *KVStore) getDocumentAnnotationsIndex(documentID string) ([]string, error) {
	ctx := context.Background()
	indexKey := s.generateKey("index", "doc:annotations:"+documentID)
	data, err := s.kv.Get(ctx, indexKey)
	if err != nil {
		return nil, err
	}
	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		return nil, fmt.Errorf("unmarshal annotations index: %w", err)
	}
	return ids, nil
}

// Reading session operations (Phase 1)

func (s *KVStore) StartSession(documentID string) (*ReadingSession, error) {
	session := &ReadingSession{
		ID:          fmt.Sprintf("session:%d", time.Now().UnixNano()),
		DocumentID:  documentID,
		StartAt:     time.Now(),
	}
	ctx := context.Background()
	key := s.generateKey("session", session.ID)
	data, err := json.Marshal(session)
	if err != nil {
		return nil, fmt.Errorf("marshal session: %w", err)
	}
	if err := s.kv.Set(ctx, key, data); err != nil {
		return nil, err
	}
	// Add to document's sessions index
	if err := s.addToDocumentSessionsIndex(documentID, session.ID); err != nil {
		// Log but don't fail
	}
	return session, nil
}

func (s *KVStore) EndSession(sessionID string, pagesRead int, notes string) error {
	ctx := context.Background()

	// Get session first
	key := s.generateKey("session", sessionID)
	data, err := s.kv.Get(ctx, key)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return err
	}
	var session ReadingSession
	if err := json.Unmarshal(data, &session); err != nil {
		return fmt.Errorf("unmarshal session: %w", err)
	}

	session.EndAt = time.Now()
	session.PagesRead = pagesRead
	session.Notes = notes

	updatedData, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	return s.kv.Set(ctx, key, updatedData)
}

func (s *KVStore) ListSessions(documentID string) ([]*ReadingSession, error) {
	ctx := context.Background()

	indexKey := s.generateKey("index", "doc:sessions:"+documentID)
	idsData, err := s.kv.Get(ctx, indexKey)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}

	var ids []string
	if err == nil {
		if err := json.Unmarshal(idsData, &ids); err != nil {
			ids = []string{}
		}
	}

	var sessions []*ReadingSession
	for _, id := range ids {
		key := s.generateKey("session", id)
		data, err := s.kv.Get(ctx, key)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				continue // Orphaned session
			}
			return nil, err
		}
		var sess ReadingSession
		if err := json.Unmarshal(data, &sess); err != nil {
			continue
		}
		sessions = append(sessions, &sess)
	}

	// Sort by StartAt descending (newest first)
	for i := 1; i < len(sessions); i++ {
		j := i
		for j > 0 && sessions[j-1].StartAt.Before(sessions[j].StartAt) {
			sessions[j-1], sessions[j] = sessions[j], sessions[j-1]
			j--
		}
	}

	return sessions, nil
}

func (s *KVStore) addToDocumentSessionsIndex(documentID, sessionID string) error {
	ctx := context.Background()
	indexKey := s.generateKey("index", "doc:sessions:"+documentID)
	ids, err := s.getDocumentSessionsIndex(documentID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	for _, id := range ids {
		if id == sessionID {
			return nil
		}
	}
	ids = append(ids, sessionID)
	data, _ := json.Marshal(ids)
	return s.kv.Set(ctx, indexKey, data)
}

func (s *KVStore) getDocumentSessionsIndex(documentID string) ([]string, error) {
	ctx := context.Background()
	indexKey := s.generateKey("index", "doc:sessions:"+documentID)
	data, err := s.kv.Get(ctx, indexKey)
	if err != nil {
		return nil, err
	}
	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		return nil, fmt.Errorf("unmarshal sessions index: %w", err)
	}
	return ids, nil
}

// Flashcard operations (Phase 2)

func (s *KVStore) AddFlashcard(card *Flashcard) error {
	if card.ID == "" {
		card.ID = fmt.Sprintf("flashcard:%d", time.Now().UnixNano())
	}
	now := time.Now()
	card.CreatedAt = now
	card.UpdatedAt = now

	ctx := context.Background()
	key := s.generateKey("flashcard", card.ID)
	data, err := json.Marshal(card)
	if err != nil {
		return fmt.Errorf("marshal flashcard: %w", err)
	}
	if err := s.kv.Set(ctx, key, data); err != nil {
		return err
	}

	// Add to flashcard index
	if err := s.addToFlashcardIndex(card.ID); err != nil {
		// Log but don't fail
	}

	return nil
}

func (s *KVStore) GetFlashcard(id string) (*Flashcard, error) {
	ctx := context.Background()
	key := s.generateKey("flashcard", id)
	data, err := s.kv.Get(ctx, key)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	var c Flashcard
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("unmarshal flashcard: %w", err)
	}
	return &c, nil
}

func (s *KVStore) ListFlashcards(opts *FlashcardListOptions) ([]*Flashcard, error) {
	ctx := context.Background()

	indexKey := s.generateKey("index", "flashcards")
	idsData, err := s.kv.Get(ctx, indexKey)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}

	var ids []string
	if err == nil {
		if err := json.Unmarshal(idsData, &ids); err != nil {
			ids = []string{}
		}
	}

	var cards []*Flashcard
	for _, id := range ids {
		card, err := s.GetFlashcard(id)
		if err != nil {
			continue
		}
		if card == nil {
			continue
		}

		// Apply filters
		if opts != nil {
			if opts.DocumentID != "" && card.DocumentID != opts.DocumentID {
				continue
			}
			if opts.Tag != "" {
				found := false
				for _, t := range card.Tags {
					if strings.EqualFold(t, opts.Tag) {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
			if opts.Due && card.DueAt.After(time.Now()) {
				continue
			}
		}

		cards = append(cards, card)

		if opts != nil && opts.Limit > 0 && len(cards) >= opts.Limit {
			break
		}
	}

	// Sort by due date ascending
	for i := 1; i < len(cards); i++ {
		j := i
		for j > 0 && cards[j-1].DueAt.After(cards[j].DueAt) {
			cards[j-1], cards[j] = cards[j], cards[j-1]
			j--
		}
	}

	return cards, nil
}

func (s *KVStore) UpdateFlashcard(card *Flashcard) error {
	existing, err := s.GetFlashcard(card.ID)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("flashcard not found: %s", card.ID)
	}

	card.CreatedAt = existing.CreatedAt
	card.UpdatedAt = time.Now()

	ctx := context.Background()
	key := s.generateKey("flashcard", card.ID)
	data, err := json.Marshal(card)
	if err != nil {
		return fmt.Errorf("marshal flashcard: %w", err)
	}
	return s.kv.Set(ctx, key, data)
}

func (s *KVStore) DeleteFlashcard(id string) error {
	ctx := context.Background()

	// Remove from index
	if err := s.removeFromFlashcardIndex(id); err != nil {
		// Log but continue
	}

	key := s.generateKey("flashcard", id)
	return s.kv.Delete(ctx, key)
}

func (s *KVStore) ReviewFlashcard(id string, quality int) (*Flashcard, error) {
	card, err := s.GetFlashcard(id)
	if err != nil {
		return nil, err
	}
	if card == nil {
		return nil, fmt.Errorf("flashcard not found: %s", id)
	}

	now := time.Now()

	// Capture previous values
	prevInterval := card.Interval
	prevEase := card.Ease
	if prevEase == 0 {
		prevEase = 2.5
	}

	// SM-2 algorithm
	ease := prevEase
	ease = ease + (0.1 - (float64(5-quality)*(0.08+float64(5-quality)*0.02)))
	if ease < 1.3 {
		ease = 1.3
	}
	if ease > 2.5 {
		ease = 2.5
	}

	var interval int
	if quality < 3 {
		interval = 1
	} else {
		if prevInterval == 0 {
			interval = 1
		} else if prevInterval == 1 {
			interval = 6
		} else {
			interval = int(float64(prevInterval) * ease)
		}
	}

	card.Interval = interval
	card.Ease = ease
	card.DueAt = now.AddDate(0, 0, interval)
	card.LastReview = now
	card.UpdatedAt = now

	// Save updated card
	if err := s.UpdateFlashcard(card); err != nil {
		return nil, err
	}

	// Record review
	review := &FlashcardReview{
		ID:            fmt.Sprintf("review:%d", time.Now().UnixNano()),
		FlashcardID:   id,
		Quality:       quality,
		ReviewedAt:   now,
		PrevInterval: prevInterval,
		PrevEase:     prevEase,
	}
	// Store review (we need a review index)
	if err := s.addReview(review); err != nil {
		// Log but don't fail
	}

	return card, nil
}

func (s *KVStore) addReview(review *FlashcardReview) error {
	ctx := context.Background()
	key := s.generateKey("review", review.ID)
	data, err := json.Marshal(review)
	if err != nil {
		return err
	}
	if err := s.kv.Set(ctx, key, data); err != nil {
		return err
	}

	// Add to flashcard's review index
	indexKey := s.generateKey("index", "flashcard:reviews:"+review.FlashcardID)
	ids, err := s.getFlashcardReviewIndex(review.FlashcardID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	ids = append(ids, review.ID)
	data, _ = json.Marshal(ids)
	return s.kv.Set(ctx, indexKey, data)
}

func (s *KVStore) getFlashcardReviewIndex(flashcardID string) ([]string, error) {
	ctx := context.Background()
	indexKey := s.generateKey("index", "flashcard:reviews:"+flashcardID)
	data, err := s.kv.Get(ctx, indexKey)
	if err != nil {
		return nil, err
	}
	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		return nil, fmt.Errorf("unmarshal review index: %w", err)
	}
	return ids, nil
}

func (s *KVStore) ListFlashcardReviews(flashcardID string) ([]*FlashcardReview, error) {
	ctx := context.Background()

	indexKey := s.generateKey("index", "flashcard:reviews:"+flashcardID)
	idsData, err := s.kv.Get(ctx, indexKey)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}

	var ids []string
	if err == nil {
		if err := json.Unmarshal(idsData, &ids); err != nil {
			ids = []string{}
		}
	}

	var reviews []*FlashcardReview
	for _, id := range ids {
		key := s.generateKey("review", id)
		data, err := s.kv.Get(ctx, key)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				continue
			}
			return nil, err
		}
		var r FlashcardReview
		if err := json.Unmarshal(data, &r); err != nil {
			continue
		}
		reviews = append(reviews, &r)
	}

	// Sort by reviewed_at descending
	for i := 1; i < len(reviews); i++ {
		j := i
		for j > 0 && reviews[j-1].ReviewedAt.Before(reviews[j].ReviewedAt) {
			reviews[j-1], reviews[j] = reviews[j], reviews[j-1]
			j--
		}
	}

	return reviews, nil
}

func (s *KVStore) GetDueFlashcards(now time.Time) ([]*Flashcard, error) {
	// Reuse ListFlashcards with due filter
	return s.ListFlashcards(&FlashcardListOptions{Due: true})
}

// Flashcard index maintenance

func (s *KVStore) addToFlashcardIndex(cardID string) error {
	ctx := context.Background()
	indexKey := s.generateKey("index", "flashcards")
	ids, err := s.getFlashcardIndex()
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	for _, id := range ids {
		if id == cardID {
			return nil
		}
	}
	ids = append(ids, cardID)
	data, _ := json.Marshal(ids)
	return s.kv.Set(ctx, indexKey, data)
}

func (s *KVStore) removeFromFlashcardIndex(cardID string) error {
	ctx := context.Background()
	indexKey := s.generateKey("index", "flashcards")
	ids, err := s.getFlashcardIndex()
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return err
	}
	newIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != cardID {
			newIDs = append(newIDs, id)
		}
	}
	data, _ := json.Marshal(newIDs)
	return s.kv.Set(ctx, indexKey, data)
}

func (s *KVStore) getFlashcardIndex() ([]string, error) {
	ctx := context.Background()
	indexKey := s.generateKey("index", "flashcards")
	data, err := s.kv.Get(ctx, indexKey)
	if err != nil {
		return nil, err
	}
	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		return nil, fmt.Errorf("unmarshal flashcard index: %w", err)
	}
	return ids, nil
}

// Task operations (Phase 3) - Stubs for KVStore
// TODO: Implement proper task support for KV backend

func (s *KVStore) AddTask(t *Task) error {
	return fmt.Errorf("tasks not yet implemented for KV store: use SQL backend")
}

func (s *KVStore) GetTask(id string) (*Task, error) {
	return nil, fmt.Errorf("tasks not yet implemented for KV store: use SQL backend")
}

func (s *KVStore) ListTasks(opts *TaskListOptions) ([]*Task, error) {
	return nil, fmt.Errorf("tasks not yet implemented for KV store: use SQL backend")
}

func (s *KVStore) UpdateTask(t *Task) error {
	return fmt.Errorf("tasks not yet implemented for KV store: use SQL backend")
}

func (s *KVStore) DeleteTask(id string) error {
	return fmt.Errorf("tasks not yet implemented for KV store: use SQL backend")
}
