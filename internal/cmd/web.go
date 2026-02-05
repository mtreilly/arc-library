// Copyright (c) 2025 Arc Engineering
// SPDX-License-Identifier: MIT

package cmd

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/mtreilly/arc-library/internal/library"
	"github.com/spf13/cobra"
	"github.com/yourorg/arc-sdk/config"
)

func newWebCmd(cfg *config.Config, store library.LibraryStore) *cobra.Command {
	var (
		port   int
		bind   string
		noOpen bool
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start web UI server",
		Long:  "Start a read-only web interface for browsing the library.",
		RunE: func(cmd *cobra.Command, args []string) error {
			addr := fmt.Sprintf("%s:%d", bind, port)

			http.HandleFunc("/", handleIndex(store))
			http.HandleFunc("/api/documents", handleAPIDocuments(store))
			http.HandleFunc("/api/search", handleAPISearch(store))
			http.HandleFunc("/api/document/", handleAPIDocument(store))
			http.HandleFunc("/document/", handleDocumentPage(store))

			fmt.Printf("Starting arc-library web server on http://%s\n", addr)
			fmt.Println("Press Ctrl+C to stop")

			return http.ListenAndServe(addr, nil)
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 8080, "Port to serve on")
	cmd.Flags().StringVarP(&bind, "bind", "b", "127.0.0.1", "Address to bind to")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "Don't open browser automatically")

	return cmd
}

var indexTemplate = `<!DOCTYPE html>
<html>
<head>
	<title>Arc Library</title>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<style>
		* { box-sizing: border-box; margin: 0; padding: 0; }
		body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; max-width: 1200px; margin: 0 auto; padding: 20px; }
		h1 { margin-bottom: 20px; color: #2c3e50; }
		.search-box { width: 100%; padding: 12px; font-size: 16px; border: 2px solid #ddd; border-radius: 4px; margin-bottom: 20px; }
		.search-box:focus { outline: none; border-color: #3498db; }
		.stats { display: flex; gap: 20px; margin-bottom: 20px; flex-wrap: wrap; }
		.stat { background: #f8f9fa; padding: 10px 20px; border-radius: 4px; }
		.stat-value { font-size: 24px; font-weight: bold; color: #3498db; }
		.stat-label { font-size: 12px; color: #666; text-transform: uppercase; }
		.documents { display: grid; gap: 15px; }
		.doc { background: white; border: 1px solid #e0e0e0; border-radius: 8px; padding: 20px; transition: box-shadow 0.2s; }
		.doc:hover { box-shadow: 0 4px 12px rgba(0,0,0,0.1); }
		.doc-title { font-size: 18px; font-weight: 600; margin-bottom: 8px; }
		.doc-title a { color: #2c3e50; text-decoration: none; }
		.doc-title a:hover { color: #3498db; }
		.doc-meta { color: #666; font-size: 14px; margin-bottom: 10px; }
		.doc-authors { color: #666; font-size: 14px; margin-bottom: 10px; }
		.doc-tags { display: flex; gap: 8px; flex-wrap: wrap; }
		.tag { background: #e3f2fd; color: #1976d2; padding: 4px 12px; border-radius: 12px; font-size: 12px; }
		.doc-abstract { color: #555; font-size: 14px; margin-top: 10px; line-height: 1.5; }
		.loading { text-align: center; padding: 40px; color: #666; }
		.error { background: #fee; color: #c33; padding: 20px; border-radius: 4px; margin: 20px 0; }
	</style>
</head>
<body>
	<h1>📚 Arc Library</h1>
	
	<div class="stats" id="stats">
		<div class="stat">
			<div class="stat-value" id="stat-count">-</div>
			<div class="stat-label">Documents</div>
		</div>
		<div class="stat">
			<div class="stat-value" id="stat-collections">-</div>
			<div class="stat-label">Collections</div>
		</div>
	</div>

	<input type="text" class="search-box" id="search" placeholder="Search documents...">
	
	<div class="documents" id="documents">
		<div class="loading">Loading documents...</div>
	</div>

	<script>
		async function loadStats() {
			try {
				const res = await fetch('/api/documents');
				const docs = await res.json();
				document.getElementById('stat-count').textContent = docs.length;
			} catch (e) {
				console.error('Failed to load stats:', e);
			}
		}

		async function loadDocuments(query = '') {
			const container = document.getElementById('documents');
			container.innerHTML = '<div class="loading">Loading...</div>';
			
			try {
				const url = query ? '/api/search?q=' + encodeURIComponent(query) : '/api/documents';
				const res = await fetch(url);
				const docs = await res.json();
				
				if (docs.length === 0) {
					container.innerHTML = '<div class="loading">No documents found</div>';
					return;
				}
				
				container.innerHTML = docs.map(function(doc) {
					var html = '<div class="doc">';
					html += '<div class="doc-title"><a href="/document/' + doc.id + '">' + escapeHtml(doc.title || 'Untitled') + '</a></div>';
					html += '<div class="doc-meta">' + doc.type + ' · ' + doc.source;
					if (doc.source_id) html += ': ' + doc.source_id;
					if (doc.rating) html += ' · ' + '⭐'.repeat(doc.rating);
					html += '</div>';
					if (doc.authors && doc.authors.length) {
						html += '<div class="doc-authors">' + escapeHtml(doc.authors.join(', ')) + '</div>';
					}
					if (doc.tags && doc.tags.length) {
						html += '<div class="doc-tags">';
						doc.tags.forEach(function(t) {
							html += '<span class="tag">' + escapeHtml(t) + '</span>';
						});
						html += '</div>';
					}
					if (doc.abstract) {
						html += '<div class="doc-abstract">' + escapeHtml(doc.abstract.substring(0, 300));
						if (doc.abstract.length > 300) html += '...';
						html += '</div>';
					}
					html += '</div>';
					return html;
				}).join('');
			} catch (e) {
				container.innerHTML = '<div class="error">Failed to load documents</div>';
			}
		}
		
		function escapeHtml(text) {
			const div = document.createElement('div');
			div.textContent = text;
			return div.innerHTML;
		}
		
		document.getElementById('search').addEventListener('input', function(e) {
			loadDocuments(e.target.value);
		});
		
		loadStats();
		loadDocuments();
	</script>
</body>
</html>
`

func handleIndex(store library.LibraryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(indexTemplate))
	}
}

func handleAPIDocuments(store library.LibraryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		docs, err := store.ListDocuments(&library.ListOptions{Limit: 100})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(docs)
	}
}

func handleAPISearch(store library.LibraryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if q == "" {
			handleAPIDocuments(store)(w, r)
			return
		}

		opts := &library.ListOptions{
			Search: q,
			Limit:  50,
		}
		docs, err := store.ListDocuments(opts)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(docs)
	}
}

func handleAPIDocument(store library.LibraryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/document/")
		doc, err := store.GetDocument(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if doc == nil {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(doc)
	}
}

func handleDocumentPage(store library.LibraryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/document/")
		doc, err := store.GetDocument(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if doc == nil {
			http.NotFound(w, r)
			return
		}

		tmpl := `<!DOCTYPE html>
<html>
<head>
	<title>{{.Title}} - Arc Library</title>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<style>
		* { box-sizing: border-box; margin: 0; padding: 0; }
		body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; max-width: 900px; margin: 0 auto; padding: 20px; }
		a { color: #3498db; }
		h1 { margin: 20px 0; color: #2c3e50; }
		.back { margin-bottom: 20px; }
		.meta { color: #666; margin-bottom: 20px; }
		.authors { font-style: italic; margin-bottom: 20px; }
		.abstract { background: #f8f9fa; padding: 20px; border-radius: 8px; margin: 20px 0; }
		.fulltext { white-space: pre-wrap; font-family: Georgia, serif; line-height: 1.8; color: #444; }
		.tags { margin: 20px 0; }
		.tag { display: inline-block; background: #e3f2fd; color: #1976d2; padding: 4px 12px; border-radius: 12px; font-size: 14px; margin-right: 8px; }
	</style>
</head>
<body>
	<div class="back"><a href="/">← Back to library</a></div>
	<h1>{{.Title}}</h1>
	<div class="meta">{{.Type}} · {{.Source}}{{if .SourceID}}: {{.SourceID}}{{end}}</div>
	{{if .Authors}}
	<div class="authors">{{join .Authors ", "}}</div>
	{{end}}
	{{if .Tags}}
	<div class="tags">
		{{range .Tags}}<span class="tag">{{.}}</span>{{end}}
	</div>
	{{end}}
	{{if .Abstract}}
	<div class="abstract">{{.Abstract}}</div>
	{{end}}
	{{if .FullText}}
	<div class="fulltext">{{.FullText}}</div>
	{{end}}
</body>
</html>`

		funcs := template.FuncMap{
			"join": strings.Join,
		}
		t := template.Must(template.New("doc").Funcs(funcs).Parse(tmpl))
		t.Execute(w, doc)
	}
}
