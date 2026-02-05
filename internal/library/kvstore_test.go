// Copyright (c) 2025 Arc Engineering
// SPDX-License-Identifier: MIT

package library

import (
	"testing"

	"github.com/yourorg/arc-sdk/store"
)

func TestKVStorePaperCRUD(t *testing.T) {
	kv := store.NewMemoryStore()
	s, err := NewKVStore(kv)
	if err != nil {
		t.Fatal(err)
	}

	// Add a paper
	paper := &Paper{
		Path:     "/tmp/test.pdf",
		Source:   "local",
		Title:    "Test Paper",
		Authors:  []string{"Alice", "Bob"},
		Abstract: "A test paper",
		Tags:     []string{"test", "demo"},
	}
	if err := s.AddPaper(paper); err != nil {
		t.Fatalf("AddPaper: %v", err)
	}
	if paper.ID == "" {
		t.Error("Paper ID should be generated")
	}

	// Retrieve by ID
	retrieved, err := s.GetPaper(paper.ID)
	if err != nil {
		t.Fatalf("GetPaper: %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetPaper returned nil")
	}
	if retrieved.Title != paper.Title {
		t.Fatalf("Title mismatch: got %q, want %q", retrieved.Title, paper.Title)
	}
	if len(retrieved.Tags) != 2 {
		t.Fatalf("Tags length: got %d, want 2", len(retrieved.Tags))
	}

	// Retrieve by path
	byPath, err := s.GetPaperByPath(paper.Path)
	if err != nil {
		t.Fatalf("GetPaperByPath: %v", err)
	}
	if byPath.ID != paper.ID {
		t.Fatalf("GetPaperByPath returned wrong paper")
	}

	// Update paper
	paper.Title = "Updated Title"
	paper.Tags = append(paper.Tags, "new")
	if err := s.UpdatePaper(paper); err != nil {
		t.Fatalf("UpdatePaper: %v", err)
	}
	updated, err := s.GetPaper(paper.ID)
	if err != nil {
		t.Fatalf("GetPaper after update: %v", err)
	}
	if updated.Title != "Updated Title" {
		t.Fatalf("Updated title not saved")
	}
	if len(updated.Tags) != 3 {
		t.Fatalf("Tags after update: got %d, want 3", len(updated.Tags))
	}

	// List papers
	papers, err := s.ListPapers(nil)
	if err != nil {
		t.Fatalf("ListPapers: %v", err)
	}
	if len(papers) != 1 {
		t.Fatalf("ListPapers returned %d papers, want 1", len(papers))
	}

	// Delete paper
	if err := s.DeletePaper(paper.ID); err != nil {
		t.Fatalf("DeletePaper: %v", err)
	}
	deleted, err := s.GetPaper(paper.ID)
	if err != nil {
		t.Fatalf("GetPaper after delete: %v", err)
	}
	if deleted != nil {
		t.Fatal("Paper still exists after delete")
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

	// Add a paper first
	paper := &Paper{
		Path:   "/tmp/test.pdf",
		Source: "local",
		Title:  "Paper for Annotations",
	}
	if err := s.AddPaper(paper); err != nil {
		t.Fatalf("AddPaper: %v", err)
	}

	// Add annotation
	ann := &Annotation{
		PaperID:  paper.ID,
		Type:     "highlight",
		Content:  "Important point",
		Page:     1,
		Position: `{"x": 10, "y": 20}`,
		Color:    "#ff0000",
	}
	if err := s.AddAnnotation(ann); err != nil {
		t.Fatalf("AddAnnotation: %v", err)
	}

	// Get annotations for paper
	anns, err := s.GetAnnotations(paper.ID)
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
	anns2, _ := s.GetAnnotations(paper.ID)
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

	// Add papers with tags
	p1 := &Paper{
		Path:   "/tmp/p1.pdf",
		Source: "local",
		Title:  "Paper 1",
		Tags:   []string{"ml", "ai"},
	}
	if err := s.AddPaper(p1); err != nil {
		t.Fatalf("AddPaper p1: %v", err)
	}

	p2 := &Paper{
		Path:   "/tmp/p2.pdf",
		Source: "local",
		Title:  "Paper 2",
		Tags:   []string{"ml", "nlp"},
	}
	if err := s.AddPaper(p2); err != nil {
		t.Fatalf("AddPaper p2: %v", err)
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

	// AddTag to paper 2
	if err := s.AddTag(p2.ID, "vision"); err != nil {
		t.Fatalf("AddTag: %v", err)
	}
	tagCounts, _ = s.ListTags()
	if tagCounts["vision"] != 1 {
		t.Fatalf("vision count after add: got %d, want 1", tagCounts["vision"])
	}

	// RemoveTag from paper 2
	if err := s.RemoveTag(p2.ID, "ml"); err != nil {
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
	paper := &Paper{
		Path:   "/tmp/persist.pdf",
		Source: "local",
		Title:  "Persistent Paper",
	}
	if err := s1.AddPaper(paper); err != nil {
		t.Fatalf("AddPaper: %v", err)
	}

	// Second store instance (simulate restart)
	s2, _ := NewKVStore(kv)
	retrieved, err := s2.GetPaper(paper.ID)
	if err != nil {
		t.Fatalf("GetPaper on new instance: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Paper not persisted across KV store instances")
	}
	if retrieved.Title != paper.Title {
		t.Fatalf("Title mismatch after persistence")
	}
}

func TestKVStoreGetPaperBySourceID(t *testing.T) {
	kv := store.NewMemoryStore()
	s, _ := NewKVStore(kv)

	paper := &Paper{
		Source:   "arxiv",
		SourceID: "2304.00067",
		Title:    "ArXiv Paper",
	}
	if err := s.AddPaper(paper); err != nil {
		t.Fatalf("AddPaper: %v", err)
	}

	retrieved, err := s.GetPaperBySourceID("arxiv", "2304.00067")
	if err != nil {
		t.Fatalf("GetPaperBySourceID: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Paper not found by source ID")
	}
	if retrieved.ID != paper.ID {
		t.Fatalf("Wrong paper returned")
	}
}

func TestKVStorePaperIndexMaintenance(t *testing.T) {
	kv := store.NewMemoryStore()
	s, _ := NewKVStore(kv)

	// Initially empty index should not error on ListPapers
	papers, err := s.ListPapers(nil)
	if err != nil {
		t.Fatalf("ListPapers on empty store: %v", err)
	}
	if len(papers) != 0 {
		t.Fatalf("Expected 0 papers, got %d", len(papers))
	}

	// Add papers and verify index updates
	p1 := &Paper{Path: "/p1", Source: "local", Title: "P1"}
	p2 := &Paper{Path: "/p2", Source: "local", Title: "P2"}
	if err := s.AddPaper(p1); err != nil {
		t.Fatalf("AddPaper p1: %v", err)
	}
	if err := s.AddPaper(p2); err != nil {
		t.Fatalf("AddPaper p2: %v", err)
	}

	all, _ := s.ListPapers(nil)
	if len(all) != 2 {
		t.Fatalf("ListPapers after adds: got %d, want 2", len(all))
	}

	// Delete one paper
	if err := s.DeletePaper(p1.ID); err != nil {
		t.Fatalf("DeletePaper: %v", err)
	}
	remaining, _ := s.ListPapers(nil)
	if len(remaining) != 1 {
		t.Fatalf("ListPapers after delete: got %d, want 1", len(remaining))
	}
	if remaining[0].ID != p2.ID {
		t.Fatalf("Wrong paper remains after delete")
	}
}
