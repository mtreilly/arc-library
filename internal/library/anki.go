// Copyright (c) 2025 Arc Engineering
// SPDX-License-Identifier: MIT

package library

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// AnkiExporter generates .apkg files for Anki
type AnkiExporter struct {
	deckName string
}

// NewAnkiExporter creates a new Anki exporter
func NewAnkiExporter(deckName string) *AnkiExporter {
	if deckName == "" {
		deckName = "Arc Library"
	}
	return &AnkiExporter{deckName: deckName}
}

// ExportCards generates an .apkg file from flashcards
func (e *AnkiExporter) ExportCards(cards []*Flashcard, w io.Writer) error {
	// Create a temporary directory for building the package
	tmpDir, err := os.MkdirTemp("", "anki-export-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "collection.anki2")

	// Create the SQLite database
	if err := e.createDatabase(dbPath, cards); err != nil {
		return fmt.Errorf("create database: %w", err)
	}

	// Create media file (empty for now, no images/audio)
	mediaPath := filepath.Join(tmpDir, "media")
	if err := os.WriteFile(mediaPath, []byte("{}"), 0644); err != nil {
		return fmt.Errorf("create media file: %w", err)
	}

	// Create zip archive
	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	// Add collection.anki2
	if err := e.addFileToZip(zipWriter, dbPath, "collection.anki2"); err != nil {
		return fmt.Errorf("add database to zip: %w", err)
	}

	// Add media
	if err := e.addFileToZip(zipWriter, mediaPath, "media"); err != nil {
		return fmt.Errorf("add media to zip: %w", err)
	}

	return nil
}

func (e *AnkiExporter) createDatabase(dbPath string, cards []*Flashcard) error {
	// Remove existing file
	os.Remove(dbPath)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	// Create tables
	schema := `
		CREATE TABLE col (
			id INTEGER PRIMARY KEY,
			crt INTEGER NOT NULL,
			mod INTEGER NOT NULL,
			scm INTEGER NOT NULL,
			ver INTEGER NOT NULL,
			dty INTEGER NOT NULL,
			usn INTEGER NOT NULL,
			ls INTEGER NOT NULL,
			conf TEXT NOT NULL,
			models TEXT NOT NULL,
			decks TEXT NOT NULL,
			dconf TEXT NOT NULL,
			tags TEXT NOT NULL
		);

		CREATE TABLE notes (
			id INTEGER PRIMARY KEY,
			guid TEXT NOT NULL,
			mid INTEGER NOT NULL,
			usn INTEGER NOT NULL,
			mod INTEGER NOT NULL,
			sfld INTEGER NOT NULL,
			csum INTEGER NOT NULL,
			flags INTEGER NOT NULL,
			data TEXT NOT NULL,
			sflds TEXT NOT NULL
		);

		CREATE TABLE cards (
			id INTEGER PRIMARY KEY,
			nid INTEGER NOT NULL,
			did INTEGER NOT NULL,
			ord INTEGER NOT NULL,
			mod INTEGER NOT NULL,
			usn INTEGER NOT NULL,
			type INTEGER NOT NULL,
			queue INTEGER NOT NULL,
			due INTEGER NOT NULL,
			ivl INTEGER NOT NULL,
			factor INTEGER NOT NULL,
			reps INTEGER NOT NULL,
			lapses INTEGER NOT NULL,
			left INTEGER NOT NULL,
			odue INTEGER NOT NULL,
			odid INTEGER NOT NULL,
			flags INTEGER NOT NULL,
			data TEXT NOT NULL
		);

		CREATE TABLE revlog (
			id INTEGER PRIMARY KEY,
			cid INTEGER NOT NULL,
			usn INTEGER NOT NULL,
			ease INTEGER NOT NULL,
			ivl INTEGER NOT NULL,
			lastIvl INTEGER NOT NULL,
			factor INTEGER NOT NULL,
			time INTEGER NOT NULL,
			type INTEGER NOT NULL
		);

		CREATE INDEX ix_cards_nid ON cards (nid);
		CREATE INDEX ix_cards_sched ON cards (did, queue, due);
		CREATE INDEX ix_revlog_cid ON revlog (cid);
		CREATE INDEX ix_notes_csum ON notes (csum);
	`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}

	// Insert collection
	now := time.Now().UnixMilli()
	deckID := int64(1)
	modelID := int64(1)

	// Collection configuration
	conf := map[string]interface{}{
		"curModel": modelID,
		"activeDecks": []int64{deckID},
	}
	confJSON, _ := json.Marshal(conf)

	// Model (note type) - basic front/back
	model := map[string]interface{}{
		fmt.Sprintf("%d", modelID): map[string]interface{}{
			"id":   modelID,
			"name": "Basic",
			"type": 0,
			"mod":  now,
			"usn":  -1,
			"sortf": 0,
			"did":  deckID,
			"tmpls": []map[string]interface{}{
				{
					"name": "Card 1",
					"ord":  0,
					"qfmt": "{{Front}}",
					"afmt": "{{FrontSide}}<hr id=\"answer\">{{Back}}",
					"bqfmt": "",
					"bafmt": "",
					"did": nil,
					"bfont": "Arial",
					"bsize": 20,
				},
			},
			"flds": []map[string]interface{}{
				{"name": "Front", "ord": 0, "sticky": false, "rtl": false, "font": "Arial", "size": 20, "media": []string{}},
				{"name": "Back", "ord": 1, "sticky": false, "rtl": false, "font": "Arial", "size": 20, "media": []string{}},
			},
			"css": ".card { font-family: arial; font-size: 20px; text-align: center; color: black; background-color: white; }",
			"latexPre": "\\documentclass[12pt]{article}\n\\usepackage[utf8]{inputenc}\n\\usepackage{amssymb,amsmath}\n\\pagestyle{empty}\n\\begin{document}",
			"latexPost": "\\end{document}",
			"latexsvg": false,
			"req": [][]interface{}{{0, "all", []int{0}}},
			"tags": []string{},
			"vers": []int{},
		},
	}
	modelsJSON, _ := json.Marshal(model)

	// Deck
	deck := map[string]interface{}{
		fmt.Sprintf("%d", deckID): map[string]interface{}{
			"id":      deckID,
			"name":    e.deckName,
			"desc":    "",
			"mod":     now,
			"usn":     -1,
			"collapsed": false,
			"browserCollapsed": false,
			"dyn":     0,
			"newToday": []interface{}{0, 0},
			"revToday": []interface{}{0, 0},
			"lrnToday": []interface{}{0, 0},
			"timeToday": []interface{}{0, 0},
			"conf":    1,
		},
	}
	decksJSON, _ := json.Marshal(deck)

	// Deck config
	dconf := map[string]interface{}{
		"1": map[string]interface{}{
			"id": 1,
			"mod": now,
			"usn": -1,
			"maxTaken": 60,
			"autoplay": true,
			"timer": 0,
			"replayq": true,
			"new": map[string]interface{}{
				"delays": []float64{1, 10},
				"ints":   []int{1, 4, 7},
				"initialFactor": 2500,
				"separate": true,
				"order": 1,
				"perDay": 20,
			},
			"rev": map[string]interface{}{
				"perDay": 200,
				"fuzz":   0.05,
				"minSpace": 1,
				"ivlFct": 1,
				"maxIvl": 36500,
			},
			"lapse": map[string]interface{}{
				"delays": []float64{10},
				"mult":   0,
				"minInt": 1,
				"leechFails": 8,
				"leechAction": 0,
			},
		},
	}
	dconfJSON, _ := json.Marshal(dconf)

	_, err = db.Exec(`
		INSERT INTO col (id, crt, mod, scm, ver, dty, usn, ls, conf, models, decks, dconf, tags)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, 1, now/1000, now, now, 11, 0, 0, 0, string(confJSON), string(modelsJSON), string(decksJSON), string(dconfJSON), "[]")

	if err != nil {
		return fmt.Errorf("insert collection: %w", err)
	}

	// Insert cards
	for i, card := range cards {
		if err := e.insertCard(db, int64(i), card, modelID, deckID, now); err != nil {
			return fmt.Errorf("insert card %d: %w", i, err)
		}
	}

	return nil
}

func (e *AnkiExporter) insertCard(db *sql.DB, idx int64, card *Flashcard, modelID, deckID, now int64) error {
	// Generate IDs
	noteID := now + idx*1000
	cardID := noteID + 1

	// Fields: Front and Back
	front := card.Front
	back := card.Back
	if card.Type == "cloze" && card.Cloze != "" {
		front = card.Cloze
		back = card.Back
	}

	fields := front + "\x1f" + back // \x1f is the field separator
	sfld := front // sort field

	// Checksum (simple hash of fields)
	csum := int64(0)
	for _, c := range fields {
		csum = (csum*31 + int64(c)) & 0xFFFFFFFF
	}

	// Insert note
	_, err := db.Exec(`
		INSERT INTO notes (id, guid, mid, usn, mod, sfld, csum, flags, data, sflds)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, noteID, fmt.Sprintf("arc-%d", noteID), modelID, -1, now, sfld, csum, 0, "", fields)

	if err != nil {
		return fmt.Errorf("insert note: %w", err)
	}

	// Calculate due date (days since collection creation)
	daysDue := 0
	if !card.DueAt.IsZero() {
		daysDue = int(time.Until(card.DueAt).Hours() / 24)
		if daysDue < 0 {
			daysDue = 0
		}
	}

	// Interval in days -> seconds for Anki
	ivl := card.Interval
	if ivl < 0 {
		ivl = 0
	}

	// Ease factor (SM-2 ease * 1000, default 2500)
	factor := int64(card.Ease * 1000)
	if factor < 1300 {
		factor = 2500
	}

	// Insert card
	_, err = db.Exec(`
		INSERT INTO cards (id, nid, did, ord, mod, usn, type, queue, due, ivl, factor, reps, lapses, left, odue, odid, flags, data)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, cardID, noteID, deckID, 0, now, -1, 0, 0, daysDue, ivl, factor, 0, 0, 0, 0, 0, 0, "")

	if err != nil {
		return fmt.Errorf("insert card: %w", err)
	}

	return nil
}

func (e *AnkiExporter) addFileToZip(zw *zip.Writer, filePath, nameInZip string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = nameInZip
	header.Method = zip.Deflate

	writer, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, file)
	return err
}
