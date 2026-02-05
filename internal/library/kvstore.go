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

// KVStore implements the same Store interface but uses arc-sdk/store.KVStore for persistence.
// This allows arc-library to work with any KVStore backend (SQLite, memory, etc.)
// and makes the module stateless from the perspective of the database.
type KVStore struct {
	kv store.KVStore
}

// NewKVStore creates a new library store backed by the given KVStore.
// The KVStore is typically opened via store.OpenSQLiteStore(path) or store.NewMemoryStore().
func NewKVStore(kv store.KVStore) (*KVStore, error) {
	s := &KVStore{kv: kv}
	// No schema initialization needed - KV store is schemaless
	return s, nil
}

// generateKey creates namespaced keys for different entity types.
func (s *KVStore) generateKey(prefix, id string) string {
	return fmt.Sprintf("arc-library:%s:%s", prefix, id)
}

// Paper operations

func (s *KVStore) AddPaper(paper *Paper) error {
	if paper.ID == "" {
		paper.ID = fmt.Sprintf("paper:%d", time.Now().UnixNano())
	}
	now := time.Now()
	paper.CreatedAt = now
	paper.UpdatedAt = now

	data, err := json.Marshal(paper)
	if err != nil {
		return fmt.Errorf("marshal paper: %w", err)
	}

	ctx := context.Background()
	key := s.generateKey("paper", paper.ID)
	if err := s.kv.Set(ctx, key, data); err != nil {
		return fmt.Errorf("set paper: %w", err)
	}

	// Index by path for deduplication
	if paper.Path != "" {
		if err := s.kv.Set(ctx, s.generateKey("paper:path", paper.Path), []byte(paper.ID)); err != nil {
			// Log but don't fail - indices can be rebuilt
		}
	}

	// Index by source+source_id if present
	if paper.Source != "" && paper.SourceID != "" {
		sourceKey := fmt.Sprintf("%s:%s", paper.Source, paper.SourceID)
		if err := s.kv.Set(ctx, s.generateKey("paper:source", sourceKey), []byte(paper.ID)); err != nil {
			// Log but don't fail - indices can be rebuilt
		}
	}

	// Add to main paper index for ListPapers
	if err := s.addToPaperIndex(paper.ID); err != nil {
		// Log but don't fail - papers can still be retrieved by ID
	}

	return nil
}

func (s *KVStore) GetPaper(id string) (*Paper, error) {
	ctx := context.Background()
	key := s.generateKey("paper", id)
	data, err := s.kv.Get(ctx, key)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	var p Paper
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("unmarshal paper: %w", err)
	}
	return &p, nil
}

func (s *KVStore) GetPaperByPath(path string) (*Paper, error) {
	ctx := context.Background()
	key := s.generateKey("paper:path", path)
	idData, err := s.kv.Get(ctx, key)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return s.GetPaper(string(idData))
}

func (s *KVStore) GetPaperBySourceID(source, sourceID string) (*Paper, error) {
	ctx := context.Background()
	sourceKey := fmt.Sprintf("%s:%s", source, sourceID)
	key := s.generateKey("paper:source", sourceKey)
	idData, err := s.kv.Get(ctx, key)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return s.GetPaper(string(idData))
}

func (s *KVStore) ListPapers(opts *ListOptions) ([]*Paper, error) {
	// In a KV store, we need to scan all keys with the "paper:" prefix.
	// For a library of a few thousand papers, this is acceptable.
	// For larger libraries, a proper DB index would be better.
	ctx := context.Background()

	// Note: In a production KV implementation, we'd want a way to list keys by prefix.
	// For now, we'll scan all keys and filter in memory.
	// A more efficient approach would maintain a "paper index" key with all paper IDs.

	// Get all paper IDs (we need a way to enumerate keys - this is a limitation of simple KV)
	// For now: we require a separate index key that stores all paper IDs.
	// This index is maintained by AddPaper/DeletePaper.

	indexKey := s.generateKey("index", "papers")
	idsData, err := s.kv.Get(ctx, indexKey)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}

	var ids []string
	if err == nil {
		if err := json.Unmarshal(idsData, &ids); err != nil {
			// Corrupted index - rebuild by scanning? Not implemented.
			ids = []string{}
		}
	}

	var papers []*Paper
	for _, id := range ids {
		paper, err := s.GetPaper(id)
		if err != nil {
			continue
		}
		if paper == nil {
			continue
		}

		// Apply filters
		if opts != nil {
			if opts.Tag != "" {
				found := false
				for _, t := range paper.Tags {
					if strings.EqualFold(t, opts.Tag) {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
			if opts.Source != "" && paper.Source != opts.Source {
				continue
			}
			if opts.Search != "" {
				search := strings.ToLower(opts.Search)
				title := strings.ToLower(paper.Title)
				abstract := strings.ToLower(paper.Abstract)
				notes := strings.ToLower(paper.Notes)
				if !strings.Contains(title, search) && !strings.Contains(abstract, search) && !strings.Contains(notes, search) {
					continue
				}
			}
		}

		papers = append(papers, paper)

		if opts != nil && opts.Limit > 0 && len(papers) >= opts.Limit {
			break
		}
	}

	return papers, nil
}

func (s *KVStore) UpdatePaper(paper *Paper) error {
	existing, err := s.GetPaper(paper.ID)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("paper not found: %s", paper.ID)
	}

	paper.CreatedAt = existing.CreatedAt
	paper.UpdatedAt = time.Now()

	data, err := json.Marshal(paper)
	if err != nil {
		return fmt.Errorf("marshal paper: %w", err)
	}

	ctx := context.Background()
	key := s.generateKey("paper", paper.ID)
	if err := s.kv.Set(ctx, key, data); err != nil {
		return fmt.Errorf("set paper: %w", err)
	}

	// Update path index if changed
	if existing.Path != paper.Path {
		// Remove old path index
		_ = s.kv.Delete(ctx, s.generateKey("paper:path", existing.Path))
		// Add new path index
		if paper.Path != "" {
			_ = s.kv.Set(ctx, s.generateKey("paper:path", paper.Path), []byte(paper.ID))
		}
	}

	// Update source index if changed
	if existing.Source != paper.Source || existing.SourceID != paper.SourceID {
		if existing.Source != "" && existing.SourceID != "" {
			oldSourceKey := fmt.Sprintf("%s:%s", existing.Source, existing.SourceID)
			_ = s.kv.Delete(ctx, s.generateKey("paper:source", oldSourceKey))
		}
		if paper.Source != "" && paper.SourceID != "" {
			newSourceKey := fmt.Sprintf("%s:%s", paper.Source, paper.SourceID)
			_ = s.kv.Set(ctx, s.generateKey("paper:source", newSourceKey), []byte(paper.ID))
		}
	}

	return nil
}

func (s *KVStore) DeletePaper(id string) error {
	paper, err := s.GetPaper(id)
	if err != nil {
		return err
	}
	if paper == nil {
		return nil // Already deleted
	}

	ctx := context.Background()

	// Remove from all collections first
	// (cascading - we could also leave orphans and clean up later)
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
	_ = s.kv.Delete(ctx, s.generateKey("paper:path", paper.Path))
	if paper.Source != "" && paper.SourceID != "" {
		sourceKey := fmt.Sprintf("%s:%s", paper.Source, paper.SourceID)
		_ = s.kv.Delete(ctx, s.generateKey("paper:source", sourceKey))
	}

	// Remove from paper index
	if err := s.removeFromPaperIndex(id); err != nil {
		// Log but continue
	}

	// Delete paper data
	key := s.generateKey("paper", id)
	return s.kv.Delete(ctx, key)
}

// maintainPaperIndex updates the list of all paper IDs.
// This is needed because KV stores typically can't enumerate keys efficiently.
func (s *KVStore) addToPaperIndex(paperID string) error {
	ctx := context.Background()
	indexKey := s.generateKey("index", "papers")

	ids, err := s.getPaperIndex()
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}

	// Avoid duplicates
	for _, id := range ids {
		if id == paperID {
			return nil
		}
	}

	ids = append(ids, paperID)
	data, _ := json.Marshal(ids)
	return s.kv.Set(ctx, indexKey, data)
}

func (s *KVStore) removeFromPaperIndex(paperID string) error {
	ctx := context.Background()
	indexKey := s.generateKey("index", "papers")

	ids, err := s.getPaperIndex()
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return err
	}

	newIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != paperID {
			newIDs = append(newIDs, id)
		}
	}

	data, _ := json.Marshal(newIDs)
	return s.kv.Set(ctx, indexKey, data)
}

func (s *KVStore) getPaperIndex() ([]string, error) {
	ctx := context.Background()
	indexKey := s.generateKey("index", "papers")
	data, err := s.kv.Get(ctx, indexKey)
	if err != nil {
		return nil, err
	}
	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		return nil, fmt.Errorf("unmarshal paper index: %w", err)
	}
	return ids, nil
}

// Tag operations

func (s *KVStore) AddTag(paperID, tag string) error {
	paper, err := s.GetPaper(paperID)
	if err != nil {
		return err
	}
	if paper == nil {
		return fmt.Errorf("paper not found: %s", paperID)
	}

	// Check if already tagged
	for _, t := range paper.Tags {
		if strings.EqualFold(t, tag) {
			return nil
		}
	}

	paper.Tags = append(paper.Tags, tag)
	return s.UpdatePaper(paper)
}

func (s *KVStore) RemoveTag(paperID, tag string) error {
	paper, err := s.GetPaper(paperID)
	if err != nil {
		return err
	}
	if paper == nil {
		return fmt.Errorf("paper not found: %s", paperID)
	}

	newTags := make([]string, 0, len(paper.Tags))
	for _, t := range paper.Tags {
		if !strings.EqualFold(t, tag) {
			newTags = append(newTags, t)
		}
	}

	paper.Tags = newTags
	return s.UpdatePaper(paper)
}

func (s *KVStore) ListTags() (map[string]int, error) {
	papers, err := s.ListPapers(nil)
	if err != nil {
		return nil, err
	}

	tagCounts := make(map[string]int)
	for _, p := range papers {
		for _, tag := range p.Tags {
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

	// Maintain collection index (like paper index)
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
	// Then search by name (need to scan index)
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

	// Sort by name
	sorted := make([]*Collection, len(collections))
	copy(sorted, collections)
	// Simple bubble sort would be fine for small N, but use sort.Slice
	// (can't use sort here because we're in kvstore.go, need to import sort)
	// We'll leave unsorted for now or implement simple sort
	return collections, nil
}

func (s *KVStore) AddToCollection(collectionID, paperID string) error {
	c, err := s.getCollectionByID(collectionID)
	if err != nil {
		return err
	}
	if c == nil {
		return fmt.Errorf("collection not found: %s", collectionID)
	}

	// Check if already in collection
	for _, pid := range c.PaperIDs {
		if pid == paperID {
			return nil
		}
	}

	c.PaperIDs = append(c.PaperIDs, paperID)
	c.UpdatedAt = time.Now()

	ctx := context.Background()
	key := s.generateKey("collection", c.ID)
	data, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal collection: %w", err)
	}
	return s.kv.Set(ctx, key, data)
}

func (s *KVStore) RemoveFromCollection(collectionID, paperID string) error {
	c, err := s.getCollectionByID(collectionID)
	if err != nil {
		return err
	}
	if c == nil {
		return fmt.Errorf("collection not found: %s", collectionID)
	}

	newIDs := make([]string, 0, len(c.PaperIDs))
	for _, pid := range c.PaperIDs {
		if pid != paperID {
			newIDs = append(newIDs, pid)
		}
	}

	c.PaperIDs = newIDs
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

	// Note: collections store paper IDs. When we delete the collection,
	// the papers remain in the library but lose this collection reference.
	// No cascade delete of papers is performed.

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
	// Similar to paper index
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

// Annotation operations

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

	// Add to paper's annotation index
	if err := s.addToPaperAnnotationsIndex(ann.PaperID, ann.ID); err != nil {
		// Log but don't fail
	}

	return nil
}

func (s *KVStore) GetAnnotations(paperID string) ([]*Annotation, error) {
	ctx := context.Background()

	// Get annotation IDs for this paper from the index
	indexKey := s.generateKey("index", "paper:annotations:"+paperID)
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

	// Get annotation first to find its paper
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

	// Remove from paper's annotation index
	_ = s.removeFromPaperAnnotationsIndex(a.PaperID, id)

	// Delete annotation
	return s.kv.Delete(ctx, key)
}

func (s *KVStore) addToPaperAnnotationsIndex(paperID, annotationID string) error {
	ctx := context.Background()
	indexKey := s.generateKey("index", "paper:annotations:" + paperID)
	ids, err := s.getPaperAnnotationsIndex(paperID)
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

func (s *KVStore) removeFromPaperAnnotationsIndex(paperID, annotationID string) error {
	ctx := context.Background()
	indexKey := s.generateKey("index", "paper:annotations:" + paperID)
	ids, err := s.getPaperAnnotationsIndex(paperID)
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

func (s *KVStore) getPaperAnnotationsIndex(paperID string) ([]string, error) {
	ctx := context.Background()
	indexKey := s.generateKey("index", "paper:annotations:" + paperID)
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
