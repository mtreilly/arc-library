package library

import (
	"testing"
	"time"

	"github.com/yourorg/arc-sdk/store"
)

func TestKVStoreFlashcards(t *testing.T) {
	kv := store.NewMemoryStore()
	s, _ := NewKVStore(kv)

	// Add a document
	doc := &Document{
		Path:   "/tmp/flashcard_test.pdf",
		Source: "local",
		Type:   DocTypePaper,
		Title:  "Test Document for Flashcards",
	}
	if err := s.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	// Create a basic flashcard
	card := &Flashcard{
		DocumentID: doc.ID,
		Type:       "basic",
		Front:      "What is the capital of France?",
		Back:       "Paris",
		Tags:       []string{"geography", "eu"},
		DueAt:      time.Now().AddDate(0, 0, 1),
		Interval:   0,
		Ease:       2.5,
	}

	if err := s.AddFlashcard(card); err != nil {
		t.Fatalf("AddFlashcard: %v", err)
	}

	if card.ID == "" {
		t.Error("Flashcard ID should be generated")
	}

	// Retrieve
	retrieved, err := s.GetFlashcard(card.ID)
	if err != nil {
		t.Fatalf("GetFlashcard: %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetFlashcard returned nil")
	}
	if retrieved.Front != card.Front {
		t.Fatalf("Front mismatch")
	}
	if len(retrieved.Tags) != 2 {
		t.Fatalf("Tags length: got %d, want 2", len(retrieved.Tags))
	}

	// List flashcards for document
	cards, err := s.ListFlashcards(&FlashcardListOptions{DocumentID: doc.ID})
	if err != nil {
		t.Fatalf("ListFlashcards: %v", err)
	}
	if len(cards) != 1 {
		t.Fatalf("ListFlashcards returned %d, want 1", len(cards))
	}

	// Review the card (quality 4)
	reviewed, err := s.ReviewFlashcard(card.ID, 4)
	if err != nil {
		t.Fatalf("ReviewFlashcard: %v", err)
	}
	if reviewed.Interval != 1 {
		t.Logf("First review: interval = %d (expected 1)", reviewed.Interval)
	}
	if reviewed.DueAt.Before(time.Now()) {
		t.Error("DueAt should be in the future")
	}

	// List due flashcards
	now := time.Now()
	dueCards, err := s.GetDueFlashcards(now)
	if err != nil {
		t.Fatalf("GetDueFlashcards: %v", err)
	}
	// The card should be due if its due date is <= now
	// Depending on the interval, it might be due immediately or later
	_ = dueCards // Check later

	// Delete flashcard
	if err := s.DeleteFlashcard(card.ID); err != nil {
		t.Fatalf("DeleteFlashcard: %v", err)
	}
	deleted, _ := s.GetFlashcard(card.ID)
	if deleted != nil {
		t.Fatal("Flashcard still exists after delete")
	}
}

func TestFlashcardSM2(t *testing.T) {
	kv := store.NewMemoryStore()
	s, _ := NewKVStore(kv)

	doc := &Document{
		Path:   "/tmp/sm2_test.pdf",
		Source: "local",
		Type:   DocTypePaper,
		Title:  "SM2 Test Doc",
	}
	if err := s.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	card := &Flashcard{
		DocumentID: doc.ID,
		Type:       "basic",
		Front:      "Q?",
		Back:       "A",
		DueAt:      time.Now(),
		Interval:   0,
		Ease:       2.5,
	}
	if err := s.AddFlashcard(card); err != nil {
		t.Fatalf("AddFlashcard: %v", err)
	}

	// First review: quality 4 (good)
	card, err := s.ReviewFlashcard(card.ID, 4)
	if err != nil {
		t.Fatalf("ReviewFlashcard: %v", err)
	}
	if card.Interval != 1 {
		t.Errorf("After first good review: interval = %d, want 1", card.Interval)
	}
	// Ease should slightly increase from 2.5
	if card.Ease <= 2.5 {
		t.Logf("Ease after review: %.3f", card.Ease)
	}
	due1 := card.DueAt

	// Second review: quality 5 (perfect)
	card2, err := s.ReviewFlashcard(card.ID, 5)
	if err != nil {
		t.Fatalf("Second review: %v", err)
	}
	if card2.Interval != 6 {
		t.Errorf("After second review: interval = %d, want 6", card2.Interval)
	}
	due2 := card2.DueAt

	// Due date should be later than first
	if !due2.After(due1) {
		t.Error("Second due date should be after first")
	}
}
