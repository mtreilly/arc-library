// Copyright (c) 2025 Arc Engineering
// SPDX-License-Identifier: MIT

package library

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yourorg/arc-sdk/store"
)

func TestKVStoreDocumentCRUD(t *testing.T) {
	kv := store.NewMemoryStore()
	s, err := NewKVStore(kv)
	if err != nil {
		t.Fatal(err)
	}

	// Add a document
	doc := &Document{
		Path:     "/tmp/test.pdf",
		Source:   "local",
		Type:     DocTypePaper,
		Title:    "Test Document",
		Authors:  []string{"Alice", "Bob"},
		Abstract: "A test document",
		Tags:     []string{"test", "demo"},
	}
	if err := s.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if doc.ID == "" {
		t.Error("Document ID should be generated")
	}

	// Retrieve by ID
	retrieved, err := s.GetDocument(doc.ID)
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetDocument returned nil")
	}
	if retrieved.Title != doc.Title {
		t.Fatalf("Title mismatch: got %q, want %q", retrieved.Title, doc.Title)
	}
	if len(retrieved.Tags) != 2 {
		t.Fatalf("Tags length: got %d, want 2", len(retrieved.Tags))
	}

	// Retrieve by path
	byPath, err := s.GetDocumentByPath(doc.Path)
	if err != nil {
		t.Fatalf("GetDocumentByPath: %v", err)
	}
	if byPath.ID != doc.ID {
		t.Fatalf("GetDocumentByPath returned wrong document")
	}

	// Update document
	doc.Title = "Updated Title"
	doc.Tags = append(doc.Tags, "new")
	if err := s.UpdateDocument(doc); err != nil {
		t.Fatalf("UpdateDocument: %v", err)
	}
	updated, err := s.GetDocument(doc.ID)
	if err != nil {
		t.Fatalf("GetDocument after update: %v", err)
	}
	if updated.Title != "Updated Title" {
		t.Fatalf("Updated title not saved")
	}
	if len(updated.Tags) != 3 {
		t.Fatalf("Tags after update: got %d, want 3", len(updated.Tags))
	}

	// List documents
	docs, err := s.ListDocuments(nil)
	if err != nil {
		t.Fatalf("ListDocuments: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("ListDocuments returned %d documents, want 1", len(docs))
	}

	// Delete document
	if err := s.DeleteDocument(doc.ID); err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}
	deleted, err := s.GetDocument(doc.ID)
	if err != nil {
		t.Fatalf("GetDocument after delete: %v", err)
	}
	if deleted != nil {
		t.Fatal("Document still exists after delete")
	}
}

func TestKVStoreCollections(t *testing.T) {
	kv := store.NewMemoryStore()
	s, err := NewKVStore(kv)
	if err != nil {
		t.Fatal(err)
	}

	// Create collection
	c, err := s.CreateCollection("My Coll", "Description")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	if c.Name != "My Coll" {
		t.Fatalf("Collection name not set")
	}

	// Get by name
	byName, err := s.GetCollection("My Coll")
	if err != nil {
		t.Fatalf("GetCollection by name: %v", err)
	}
	if byName.ID != c.ID {
		t.Fatalf("GetCollection returned different collection")
	}

	// Get by ID
	byID, err := s.GetCollection(c.ID)
	if err != nil {
		t.Fatalf("GetCollection by ID: %v", err)
	}
	if byID.Name != "My Coll" {
		t.Fatalf("GetCollection by ID returned wrong collection")
	}

	// List collections
	collections, err := s.ListCollections()
	if err != nil {
		t.Fatalf("ListCollections: %v", err)
	}
	if len(collections) != 1 {
		t.Fatalf("ListCollections returned %d, want 1", len(collections))
	}

	// Delete collection
	if err := s.DeleteCollection(c.ID); err != nil {
		t.Fatalf("DeleteCollection: %v", err)
	}
	afterDel, _ := s.GetCollection(c.ID)
	if afterDel != nil {
		t.Fatal("Collection still exists after delete")
	}
}

func TestKVStoreAnnotations(t *testing.T) {
	kv := store.NewMemoryStore()
	s, err := NewKVStore(kv)
	if err != nil {
		t.Fatal(err)
	}

	// Add a document first
	doc := &Document{
		Path:   "/tmp/test.pdf",
		Source: "local",
		Type:   DocTypePaper,
		Title:  "Document for Annotations",
	}
	if err := s.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	// Add annotation
	ann := &Annotation{
		DocumentID: doc.ID,
		Type:       "highlight",
		Content:    "Important point",
		Page:       1,
		Position:   `{"x": 10, "y": 20}`,
		Color:      "#ff0000",
	}
	if err := s.AddAnnotation(ann); err != nil {
		t.Fatalf("AddAnnotation: %v", err)
	}

	// Get annotations for document
	anns, err := s.GetAnnotations(doc.ID)
	if err != nil {
		t.Fatalf("GetAnnotations: %v", err)
	}
	if len(anns) != 1 {
		t.Fatalf("GetAnnotations returned %d, want 1", len(anns))
	}
	if anns[0].Content != "Important point" {
		t.Fatalf("Annotation content mismatch")
	}

	// Delete annotation
	if err := s.DeleteAnnotation(ann.ID); err != nil {
		t.Fatalf("DeleteAnnotation: %v", err)
	}
	anns2, _ := s.GetAnnotations(doc.ID)
	if len(anns2) != 0 {
		t.Fatalf("Annotations after delete: got %d, want 0", len(anns2))
	}
}

func TestKVStoreTags(t *testing.T) {
	kv := store.NewMemoryStore()
	s, err := NewKVStore(kv)
	if err != nil {
		t.Fatal(err)
	}

	// Add documents with tags
	d1 := &Document{
		Path:   "/tmp/d1.pdf",
		Source: "local",
		Type:   DocTypePaper,
		Title:  "Document 1",
		Tags:   []string{"ml", "ai"},
	}
	if err := s.AddDocument(d1); err != nil {
		t.Fatalf("AddDocument d1: %v", err)
	}

	d2 := &Document{
		Path:   "/tmp/d2.pdf",
		Source: "local",
		Type:   DocTypePaper,
		Title:  "Document 2",
		Tags:   []string{"ml", "nlp"},
	}
	if err := s.AddDocument(d2); err != nil {
		t.Fatalf("AddDocument d2: %v", err)
	}

	// List tags
	tagCounts, err := s.ListTags()
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	if tagCounts["ml"] != 2 {
		t.Fatalf("ml count: got %d, want 2", tagCounts["ml"])
	}
	if tagCounts["ai"] != 1 {
		t.Fatalf("ai count: got %d, want 1", tagCounts["ai"])
	}

	// AddTag to document 2
	if err := s.AddTag(d2.ID, "vision"); err != nil {
		t.Fatalf("AddTag: %v", err)
	}
	tagCounts, _ = s.ListTags()
	if tagCounts["vision"] != 1 {
		t.Fatalf("vision count after add: got %d, want 1", tagCounts["vision"])
	}

	// RemoveTag from document 2
	if err := s.RemoveTag(d2.ID, "ml"); err != nil {
		t.Fatalf("RemoveTag: %v", err)
	}
	tagCounts, _ = s.ListTags()
	if tagCounts["ml"] != 1 {
		t.Fatalf("ml count after remove: got %d, want 1", tagCounts["ml"])
	}
}

func TestKVStorePersistence(t *testing.T) {
	// Test that data persists across store instances using the same underlying KV
	kv := store.NewMemoryStore()

	// First store instance
	s1, _ := NewKVStore(kv)
	doc := &Document{
		Path:   "/tmp/persist.pdf",
		Source: "local",
		Type:   DocTypePaper,
		Title:  "Persistent Document",
	}
	if err := s1.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	// Second store instance (simulate restart)
	s2, _ := NewKVStore(kv)
	retrieved, err := s2.GetDocument(doc.ID)
	if err != nil {
		t.Fatalf("GetDocument on new instance: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Document not persisted across KV store instances")
	}
	if retrieved.Title != doc.Title {
		t.Fatalf("Title mismatch after persistence")
	}
}

func TestKVStoreGetDocumentBySourceID(t *testing.T) {
	kv := store.NewMemoryStore()
	s, _ := NewKVStore(kv)

	doc := &Document{
		Source:   "arxiv",
		SourceID: "2304.00067",
		Type:     DocTypePaper,
		Title:    "ArXiv Document",
	}
	if err := s.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	retrieved, err := s.GetDocumentBySourceID("arxiv", "2304.00067")
	if err != nil {
		t.Fatalf("GetDocumentBySourceID: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Document not found by source ID")
	}
	if retrieved.ID != doc.ID {
		t.Fatalf("Wrong document returned")
	}
}

func TestKVStoreDocumentIndexMaintenance(t *testing.T) {
	kv := store.NewMemoryStore()
	s, _ := NewKVStore(kv)

	// Initially empty index should not error on ListDocuments
	docs, err := s.ListDocuments(nil)
	if err != nil {
		t.Fatalf("ListDocuments on empty store: %v", err)
	}
	if len(docs) != 0 {
		t.Fatalf("Expected 0 documents, got %d", len(docs))
	}

	// Add documents and verify index updates
	d1 := &Document{
		Path:   "/d1",
		Source: "local",
		Type:   DocTypePaper,
		Title:  "D1",
	}
	d2 := &Document{
		Path:   "/d2",
		Source: "local",
		Type:   DocTypePaper,
		Title:  "D2",
	}
	if err := s.AddDocument(d1); err != nil {
		t.Fatalf("AddDocument d1: %v", err)
	}
	if err := s.AddDocument(d2); err != nil {
		t.Fatalf("AddDocument d2: %v", err)
	}

	all, _ := s.ListDocuments(nil)
	if len(all) != 2 {
		t.Fatalf("ListDocuments after adds: got %d, want 2", len(all))
	}

	// Delete one document
	if err := s.DeleteDocument(d1.ID); err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}
	remaining, _ := s.ListDocuments(nil)
	if len(remaining) != 1 {
		t.Fatalf("ListDocuments after delete: got %d, want 1", len(remaining))
	}
	if remaining[0].ID != d2.ID {
		t.Fatalf("Wrong document remains after delete")
	}
}

func TestKVStoreReadingSessions(t *testing.T) {
	kv := store.NewMemoryStore()
	s, _ := NewKVStore(kv)

	doc := &Document{
		Path:   "/tmp/session_test.pdf",
		Source: "local",
		Type:   DocTypePaper,
		Title:  "Session Test",
	}
	if err := s.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	// Start a session
	sess, err := s.StartSession(doc.ID)
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	if sess.DocumentID != doc.ID {
		t.Fatalf("Session document ID mismatch")
	}
	if !sess.EndAt.IsZero() {
		t.Fatalf("Session EndAt should be zero initially, got %v", sess.EndAt)
	}

	// End the session
	if err := s.EndSession(sess.ID, 10, "Read chapter 1"); err != nil {
		t.Fatalf("EndSession: %v", err)
	}

	// Debug: directly fetch the session from the KV store to check EndAt
	key := s.generateKey("session", sess.ID)
	raw, err := s.kv.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("raw Get session: %v", err)
	}
	t.Logf("Raw session JSON after EndSession: %s", raw)
	var directSession ReadingSession
	if err := json.Unmarshal(raw, &directSession); err != nil {
		t.Fatalf("unmarshal direct: %v", err)
	}
	t.Logf("Direct session EndAt: %v (zero: %v)", directSession.EndAt, directSession.EndAt.IsZero())

	// List sessions for document
	sessions, err := s.ListSessions(doc.ID)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("ListSessions returned %d, want 1", len(sessions))
	}
	if sessions[0].PagesRead != 10 {
		t.Fatalf("PagesRead: got %d, want 10", sessions[0].PagesRead)
	}
	if sessions[0].Notes != "Read chapter 1" {
		t.Fatalf("Notes mismatch: got %q", sessions[0].Notes)
	}
	if sessions[0].EndAt.IsZero() {
		t.Fatalf("Session EndAt is zero, want non-zero. Full session: %+v", sessions[0])
	}
}
