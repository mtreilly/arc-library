// Copyright (c) 2025 Arc Engineering
// SPDX-License-Identifier: MIT

package library

// LibraryStore is the interface for persisting and retrieving library data.
// Implementations may use SQL, KV storage, or in-memory structures.
type LibraryStore interface {
	// Paper operations
	AddPaper(*Paper) error
	GetPaper(id string) (*Paper, error)
	GetPaperByPath(path string) (*Paper, error)
	GetPaperBySourceID(source, sourceID string) (*Paper, error)
	ListPapers(opts *ListOptions) ([]*Paper, error)
	UpdatePaper(*Paper) error
	DeletePaper(id string) error

	// Tag operations
	AddTag(paperID, tag string) error
	RemoveTag(paperID, tag string) error
	ListTags() (map[string]int, error)

	// Collection operations
	CreateCollection(name, description string) (*Collection, error)
	GetCollection(idOrName string) (*Collection, error)
	ListCollections() ([]*Collection, error)
	AddToCollection(collectionID, paperID string) error
	RemoveFromCollection(collectionID, paperID string) error
	DeleteCollection(id string) error

	// Annotation operations
	AddAnnotation(*Annotation) error
	GetAnnotations(paperID string) ([]*Annotation, error)
	DeleteAnnotation(id string) error
}
