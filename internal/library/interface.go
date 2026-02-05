// Copyright (c) 2025 Arc Engineering
// SPDX-License-Identifier: MIT

package library

// LibraryStore is the interface for persisting and retrieving library data.
// Implementations may use SQL, KV storage, or in-memory structures.
type LibraryStore interface {
	// Document operations
	AddDocument(*Document) error
	GetDocument(id string) (*Document, error)
	GetDocumentByPath(path string) (*Document, error)
	GetDocumentBySourceID(source, sourceID string) (*Document, error)
	ListDocuments(opts *ListOptions) ([]*Document, error)
	UpdateDocument(*Document) error
	DeleteDocument(id string) error

	// Tag operations
	AddTag(documentID, tag string) error
	RemoveTag(documentID, tag string) error
	ListTags() (map[string]int, error)

	// Collection operations
	CreateCollection(name, description string) (*Collection, error)
	GetCollection(idOrName string) (*Collection, error)
	ListCollections() ([]*Collection, error)
	AddToCollection(collectionID, documentID string) error
	RemoveFromCollection(collectionID, documentID string) error
	DeleteCollection(id string) error

	// Annotation operations
	AddAnnotation(*Annotation) error
	GetAnnotations(documentID string) ([]*Annotation, error)
	DeleteAnnotation(id string) error

	// Reading session operations (Phase 1)
	StartSession(documentID string) (*ReadingSession, error)
	EndSession(sessionID string, pagesRead int, notes string) error
	ListSessions(documentID string) ([]*ReadingSession, error)
}
