# arc-library

A CLI tool to organize, tag, and annotate your research paper library. Works with papers from any source (arXiv, local PDFs, etc.).

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

## Usage

### Import Papers

Import papers from your filesystem into the library database:

```bash
# Import all papers from a directory
arc-library import ~/papers

# Import with tags
arc-library import ~/papers --tag ml --tag transformers

# Import into a collection
arc-library import ~/papers --collection "project-x"
```

Papers are expected to have a `meta.yaml` file (as created by [arc-arxiv](https://github.com/mtreilly/arc-arxiv)).

### Tagging

```bash
# Add tags to a paper
arc-library tag add 2304.00067 ml transformers attention

# Remove tags
arc-library tag remove 2304.00067 attention

# List all tags with counts
arc-library tag list
```

### Collections

Group papers into named collections for projects:

```bash
# Create a collection
arc-library collection create "thesis-chapter-3" --description "Papers for chapter 3"

# List collections
arc-library collection list

# Add papers to a collection
arc-library collection add "thesis-chapter-3" 2304.00067 2301.12345

# Show papers in a collection
arc-library collection show "thesis-chapter-3"

# Remove papers from a collection
arc-library collection remove "thesis-chapter-3" 2304.00067

# Delete a collection
arc-library collection delete "thesis-chapter-3" --force
```

### Listing and Searching

```bash
# List all papers
arc-library list

# Filter by tag
arc-library list --tag ml

# Filter by source
arc-library list --source arxiv

# Search across titles, abstracts, and notes
arc-library search "transformer attention"

# Search with filters
arc-library search "neural" --tag ml --limit 20
```

### Annotations

Add notes, highlights, and bookmarks to papers:

```bash
# Add a note
arc-library annotate add 2304.00067 "Key insight about attention mechanisms"

# Add with page number
arc-library annotate add 2304.00067 "Important formula" --page 5

# Add a bookmark
arc-library annotate add 2304.00067 "TODO: follow up on this" --type bookmark

# List annotations for a paper
arc-library annotate list 2304.00067

# Delete an annotation
arc-library annotate delete <annotation-id>
```

## Data Storage

arc-library supports multiple storage backends through the `ARC_LIBRARY_STORAGE` environment variable:

- **`sql`** (default): Traditional relational storage using custom SQLite schema. Supports advanced queries and indexes. Recommended for large libraries.
- **`kv`**: Key-value store using JSON documents in SQLite. Simpler schema, but list operations scan all papers (suitable for small libraries, <1000 papers).
- **`memory`**: In-memory only, no persistence. Useful for ephemeral sessions or testing.

All backends use the same default SQLite file (`~/.local/share/arc/arc.db`) unless configured otherwise. The actual paper files (PDFs, meta.yaml) remain on the filesystem - arc-library only indexes and enriches them.

### Examples

```bash
# Use KV store (JSON documents)
ARC_LIBRARY_STORAGE=kv arc-library list

# Use in-memory store (stateless)
ARC_LIBRARY_STORAGE=memory arc-library list

# Default (SQL)
arc-library list
```

## Related Tools

- [arc-arxiv](https://github.com/mtreilly/arc-arxiv) - Fetch papers from arXiv with rich metadata

## License

MIT
