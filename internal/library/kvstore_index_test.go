package library

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yourorg/arc-sdk/store"
)

func TestKVStoreDocumentIndex(t *testing.T) {
	kv := store.NewMemoryStore()
	s, _ := NewKVStore(kv)

	doc := &Document{
		Path:   "/p1",
		Source: "local",
		Title:  "Document 1",
	}
	if err := s.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	// Directly check the index key
	indexKey := s.generateKey("index", "documents")
	data, err := kv.Get(context.Background(), indexKey)
	if err != nil {
		t.Fatalf("Get index: %v", err)
	}
	t.Logf("Index data: %s", data)

	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		t.Fatalf("Unmarshal index: %v", err)
	}
	if len(ids) != 1 {
		t.Fatalf("Index has %d IDs, want 1", len(ids))
	}
	if ids[0] != doc.ID {
		t.Fatalf("Index contains wrong ID: %s", ids[0])
	}
}
