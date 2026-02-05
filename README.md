# arc-library

A CLI tool to organize, tag, and annotate your research library. Works with documents of any type: research papers, books, articles, videos, notes, and more. Designed for personal knowledge management, research, and learning.

## Features

- **Multiple document types**: papers, books, articles, videos, notes, code repos
- **Flexible import**: from directories (with meta.yaml), direct PDF import, DOI resolution
- **Full-text search**: SQLite FTS5 for fast text search (SQL backend)
- **PDF text extraction**: optional full-text indexing (requires `pdftotext`)
- **Metadata enrichment**: auto-fill metadata from Crossref via DOI
- **Tags & collections**: organize documents by topic or project
- **Annotations**: highlights, notes, bookmarks with page/position
- **Reading sessions**: track time spent reading, pages per session
- **Statistics**: overview of your library usage
- **Multiple storage backends**: SQL (default), KV (JSON), or memory (stateless)
- **CLI-first, composable**: integrates with other arc tools

## Installation

```bash
go install github.com/mtreilly/arc-library@latest
```

Or build from source:

```bash
git clone https://github.com/mtreilly/arc-library.git
cd arc-library
go build -o arc-library .
```

## Quick Start

### Import Documents

#### From a meta directory (created by arc-arxiv)

```bash
arc-library import ~/papers/2304.00067
arc-library import ~/papers --tag ml --collection "thesis"
```

#### Import a PDF directly

```bash
arc-library import paper.pdf --title "My Paper" --authors "Alice, Bob" --tag ml
```

With full-text extraction and DOI resolution:

```bash
arc-library import paper.pdf --extract-text --doi 10.1234/5678 --resolve-doi
```

You can also import all PDFs in a directory:

```bash
arc-library import ~/downloads --extract-text --tag unread
```

### Organize

```bash
# Tag documents
arc-library tag add <doc-id> ml nlp attention
arc-library tag remove <doc-id> obsolete

# Create and manage collections
arc-library collection create "project-x" --description "Papers for project X"
arc-library collection add "project-x" <doc-id>
arc-library collection show "project-x"
```

### Search & Discover

```bash
# List documents
arc-library list --tag ml --source arxiv

# Full-text search (SQL backend)
arc-library search "transformer attention" --limit 20

# Find documents by metadata
arc-library list --type book
```

### Annotate

```bash
# Add an annotation
arc-library annotate add <doc-id> "Important insight" --page 12 --color "#ff0000"

# List annotations for a document
arc-library annotate list <doc-id>

# Delete annotation
arc-library annotate delete <annotation-id>
```

### Track Reading

```bash
# Start a reading session
arc-library session start <doc-id>

# End the session (record pages read, notes)
arc-library session end <session-id> --pages 10 --notes "Read intro"

# List sessions
arc-library session list --document <doc-id>
arc-library session list --limit 10
```

### Statistics

```bash
arc-library stats
```

Shows document counts by type, tag cloud size, collections, annotations, reading sessions, pages read.

## Document Types

- `paper`: arXiv, conference, journal articles (default)
- `book`: textbooks, monographs
- `article`: web articles, blog posts
- `video`: lecture videos, tutorials
- `note`: user-created notes (Markdown, text)
- `repo`: git repositories
- `other`: anything else

Specify with `--type` flag when importing.

## PDF Import Options

- `--extract-text`: extract full text using `pdftotext` (poppler-utils). Enables full-text search.
- `--doi <doi>`: assign a DOI to the document (e.g., `10.1234/5678`)
- `--resolve-doi`: fetch metadata from Crossref (requires `--doi`)
- `--title`, `--authors`, `--abstract`: manual metadata (otherwise filename used)

## Storage Backends

Control with `ARC_LIBRARY_STORAGE` environment variable:

- `sql` (default): Relational SQLite schema with FTS5. Best performance for large libraries (>10k docs).
- `kv`: JSON documents in a key-value store. Simpler, portable, good for small libraries (<1k docs).
- `memory`: In-memory only. Useful for quick queries or when persistence not needed.

The default SQLite file is at `~/.local/share/arc/arc.db`.

## Data Model

- **Documents**: core entity, with flexible metadata (type, source, source_id, title, authors, abstract, full_text, tags, notes, rating, status, meta)
- **Collections**: named groups of documents (many-to-many)
- **Annotations**: per-document highlights/notes with position and color
- **ReadingSessions**: start/end timestamps, pages read, notes
- **Tags**: simple string tags, counted across library

## Example Workflows

### Literature review

```bash
# Import all papers from a directory (with meta.yaml)
arc-library import ~/arxiv-papers --tag literature-review

# Search for relevant papers
arc-library search "neural networks" --tag literature-review

# Create a collection for the review
arc-library collection create "lit-req-2025"
arc-library collection add "lit-req-2025" <paper-id>

# Add notes as you read
arc-library annotate add <paper-id> "Key contribution: ..." --page 3
```

### Student learning

```bash
# Import lecture PDFs
arc-library import ~/lectures --extract-text --tag course

# Track reading progress
arc-library session start <lecture-id>
# ... read ...
arc-library session end <session-id> --pages 5 --notes "Understood main concepts"

# Generate flashcards (future)
# arc-library flashcard generate --from-annotations

# View stats to see progress
arc-library stats
```

### Research with books

```bash
# Import a book (PDF or epub)
arc-library import book.pdf --type book --title "Deep Learning" --authors "Goodfellow et al." --tag reference

# Create a project collection
arc-library collection create "thesis-chapter-2"

# Add relevant chapters or notes as you read
```

## Advanced Usage

### Full-text search

After importing PDFs with `--extract-text`, use the `search` command to find content anywhere in the full text:

```bash
arc-library search "backpropagation" --type paper
```

This searches titles, abstracts, notes, **and** the extracted full text.

### Crossref DOI resolution

If you have a DOI, you can auto-populate metadata:

```bash
arc-library import paper.pdf --doi 10.1234/5678 --resolve-doi
```

This fetches title, authors, abstract, and publication year.

### Flashcards (Spaced Repetition)

Transform your annotations or create new cards for active recall learning:

```bash
# Create a basic flashcard
arc-library flashcard add --document <doc-id> --front "What is the capital of France?" --back "Paris" --tag geography

# Create a cloze deletion card
arc-library flashcard add --document <doc-id> --type cloze --cloze "The capital of France is {{c1::Paris}}" --tag geography

# List all due cards
arc-library flashcard due

# Review a card (rate recall 0-5)
arc-library flashcard review <card-id> --quality 4

# List all cards for a document
arc-library flashcard list --document <doc-id>

# Delete a card
arc-library flashcard delete <card-id>
```

The flashcard system uses the SM-2 algorithm (like Anki) to schedule reviews. Cards automatically update their due date based on your rating quality.

### AI Analysis

Leverage the arc-ai daemon with your Pi-agent to get summaries and answers about your documents:

```bash
# Generate a summary of a document
arc-library ai summary <doc-id>

# Optionally store the summary in document metadata
arc-library ai summary <doc-id> --store

# Ask a question about a document
arc-library ai qna <doc-id> "What is the main contribution of this paper?"

# Combine with full-text extraction
arc-library import paper.pdf --extract-text
arc-library ai summary <doc-id>
```

Make sure `arc-ai` is running in daemon mode: `arc-ai start`

### Reading goals

Use `arc-library stats` to see how much you've been reading:

```
Documents:     142
By type:       paper: 120, book: 15, article: 7
Tags:          23 unique
Collections:   5
Annotations:   87
Reading sessions: 42
Pages read:    1234
```

### Back up your library

The database file is a single SQLite file. Copy it to back up:

```bash
cp ~/.local/share/arc/arc.db ~/backups/arc-$(date +%F).db
```

Your actual document files remain on the filesystem; the library only stores metadata and indexes.

## Related Tools

- [arc-arxiv](https://github.com/mtreilly/arc-arxiv) - Fetch papers from arXiv with meta.yaml
- arc-ai (upcoming) - AI-powered summarization and Q&A

## Design Principles

- **Stateless modules**: Storage is optional; can run entirely in-memory
- **CLI-first**: All features accessible from the command line
- **Composable**: Works with standard Unix tools (grep, find, etc.)
- **Offline-first**: No mandatory cloud APIs; your data stays on your machine
- **Minimal dependencies**: Uses system tools (pdftotext) optionally

## License

MIT
