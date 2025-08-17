package xref

import (
	"context"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

type goAdapter struct {
	qDefs, qRefs, qImport *sitter.Query
}

// newGoAdapter creates a Go language adapter with pre-compiled tree-sitter queries.
// Loads definition, reference, and import queries from .scm files for Go syntax trees.
func newGoAdapter() (LanguageAdapter, error) {
	tsLang := golang.GetLanguage()
	
	// Load tree-sitter query for finding definitions (functions, types, vars, consts)
	qd, err := loadQuery("go", "defs.scm", tsLang)
	if err != nil {
		return nil, err
	}
	
	// Load tree-sitter query for finding references (identifier usage)
	qr, err := loadQuery("go", "refs.scm", tsLang)
	if err != nil {
		return nil, err
	}
	
	// Load tree-sitter query for finding import statements
	qi, err := loadQuery("go", "imports.scm", tsLang)
	if err != nil {
		return nil, err
	}
	
	return &goAdapter{qDefs: qd, qRefs: qr, qImport: qi}, nil
}
func (g *goAdapter) Lang() string { return "go" }
func (g *goAdapter) CanHandle(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".go")
}

func (g *goAdapter) Parse(_ string, src []byte) (*sitter.Tree, error) {
	p := sitter.NewParser()
	lang := golang.GetLanguage()
	if lang == nil {
		return nil, nil // Language not available
	}
	p.SetLanguage(lang)
	return p.ParseCtx(context.Background(), nil, src)
}

// Extract analyzes a Go source file's syntax tree and extracts all symbols.
// Uses tree-sitter queries to find imports, definitions, and references.
// Returns a FileIndex containing all discovered symbols and their locations.
func (g *goAdapter) Extract(path string, src []byte, tree *sitter.Tree) (*FileIndex, error) {
	fi := &FileIndex{Lang: "go", File: path, Defs: map[string]DefLocation{}, Refs: map[string][]RefLocation{}, Imports: map[string]string{}}
	if tree == nil {
		return fi, nil // Return empty index if parsing failed
	}
	root := tree.RootNode()
	
	// Extract import statements using the imports query
	execQuery(src, root, g.qImport, func(capts []sitter.QueryCapture, _ func(id uint32) string) {
		alias := getByName(src, capts, g.qImport, "alias")
		ipath := strings.Trim(getByName(src, capts, g.qImport, "path"), "`\"")
		rng := rangeByName(src, capts, g.qImport, "rng")
		if ipath != "" {
			// Use last path component as alias if not explicitly specified
			if alias == "" {
				alias = filepath.Base(ipath)
			}
			fi.Imports[alias] = ipath
			fi.Occurrences = append(fi.Occurrences, Occurrence{Name: alias, KindHint: "import", Rng: rng})
		}
	})
	
	// Extract definitions (functions, methods, types, variables, constants) using the defs query
	execQuery(src, root, g.qDefs, func(capts []sitter.QueryCapture, _ func(id uint32) string) {
		var name, kind, recv string
		
		// Determine the type of definition based on which query capture matched
		switch {
		case getByName(src, capts, g.qDefs, "fname") != "" && getByName(src, capts, g.qDefs, "mrecv") == "":
			// Regular function
			name, kind = getByName(src, capts, g.qDefs, "fname"), "func"
		case getByName(src, capts, g.qDefs, "fname") != "" && getByName(src, capts, g.qDefs, "mrecv") != "":
			// Method with receiver
			name, kind, recv = getByName(src, capts, g.qDefs, "fname"), "func", getByName(src, capts, g.qDefs, "mrecv")
			recv = strings.TrimPrefix(recv, "*") // Remove pointer indicator
		case getByName(src, capts, g.qDefs, "tname") != "":
			// Type definition
			name, kind = getByName(src, capts, g.qDefs, "tname"), "type"
		case getByName(src, capts, g.qDefs, "vname") != "":
			// Variable declaration
			name, kind = getByName(src, capts, g.qDefs, "vname"), "var"
		default:
			// Constant declaration
			name, kind = getByName(src, capts, g.qDefs, "cname"), "const"
		}
		if name == "" {
			return
		}
		
		// Create unique symbol ID and store the definition
		rng := rangeByName(src, capts, g.qDefs, "rng")
		sid := symbolID("go", path, recv, name)
		fi.Defs[sid] = DefLocation{Lang: "go", File: path, Rng: rng, Name: name, Kind: kind}
		fi.Occurrences = append(fi.Occurrences, Occurrence{Name: name, KindHint: "def", Rng: rng, SymbolID: sid})
	})
	
	// Extract all identifier references using the refs query
	execQuery(src, root, g.qRefs, func(capts []sitter.QueryCapture, _ func(id uint32) string) {
		id := getByName(src, capts, g.qRefs, "id")
		rng := rangeByName(src, capts, g.qRefs, "rng")
		if id != "" {
			// Store as occurrence without symbol ID (resolved later during queries)
			fi.Occurrences = append(fi.Occurrences, Occurrence{Name: id, KindHint: "ref", Rng: rng})
		}
	})
	return fi, nil
}

// ResolveAt resolves a symbol occurrence to candidate symbol IDs for "go to definition".
// First tries to find a local definition in the same file, then falls back to global name lookup.
// Returns symbol IDs in priority order (local definitions first, then global matches).
func (g *goAdapter) ResolveAt(path string, _ []byte, occ Occurrence, pi *ProjectIndex) []string {
	pi.mu.RLock()
	defer pi.mu.RUnlock()
	
	// First priority: look for a definition in the same file (local scope)
	for sid, d := range pi.Defs {
		if d.Lang == "go" && d.File == path && d.Name == occ.Name {
			return []string{sid}
		}
	}
	
	// Fallback: use global name lookup to find symbols across all files
	return append([]string(nil), pi.NameLookup["go:"+occ.Name]...)
}
