package xref

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
)

type Tree = sitter.Tree // re-export for adapter signatures

type Occurrence struct {
	Name     string
	KindHint string // "def" | "ref" | "import"
	Rng      Range
	SymbolID string // optional, set by adapter if known
}

type FileIndex struct {
	Lang        string
	File        string
	Defs        map[string]DefLocation   // SymbolID -> def
	Refs        map[string][]RefLocation // SymbolID -> refs (optional)
	Occurrences []Occurrence
	Imports     map[string]string // alias -> path/module (adapter-specific)
}

type ProjectIndex struct {
	mu         sync.RWMutex
	Defs       map[string]DefLocation
	Refs       map[string][]RefLocation
	NameLookup map[string][]string // lang:name -> []SymbolID
	FileOcc    map[string][]Occurrence
}

func newProjectIndex() *ProjectIndex {
	return &ProjectIndex{
		Defs:       map[string]DefLocation{},
		Refs:       map[string][]RefLocation{},
		NameLookup: map[string][]string{},
		FileOcc:    map[string][]Occurrence{},
	}
}

// merge combines a file's symbol index into the global project index.
// Thread-safe operation that updates definitions, references, name lookups, and file occurrences.
// Called during indexing to build the complete cross-reference database.
func (pi *ProjectIndex) merge(fi *FileIndex) {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	// Add all definitions from this file to the global definition map
	for sid, d := range fi.Defs {
		pi.Defs[sid] = d
		// Build reverse lookup: language:name -> []symbolID for fast name-based searches
		key := d.Lang + ":" + d.Name
		pi.NameLookup[key] = append(pi.NameLookup[key], sid)
	}

	// Merge reference locations for each symbol
	for sid, refs := range fi.Refs {
		pi.Refs[sid] = append(pi.Refs[sid], refs...)
	}

	// Store all raw occurrences for this file (used for cursor-based lookups)
	pi.FileOcc[fi.File] = append(pi.FileOcc[fi.File], fi.Occurrences...)
}

// New creates a new cross-reference engine with the specified language adapters.
// If no adapters are provided, it automatically registers Go, TypeScript, and Python adapters.
// Returns an Engine ready for indexing and querying code symbols.
func New(adapters ...LanguageAdapter) (*Engine, error) {
	if len(adapters) == 0 {
		// Initialize default language adapters for Go, TypeScript, and Python
		py, err := newPyAdapter()
		if err != nil {
			return nil, err
		}
		ts, err := newTsAdapter()
		if err != nil {
			return nil, err
		}
		g, err := newGoAdapter()
		if err != nil {
			return nil, err
		}
		adapters = []LanguageAdapter{g, ts, py}
	}
	return &Engine{Index: newProjectIndex(), Adapters: adapters}, nil
}

func (e *Engine) IndexRoot(root string) error {
	return e.IndexPaths(root)
}

// IndexPaths indexes the specified files and directories, building a complete symbol database.
// Uses concurrent processing with file discovery and parsing happening in parallel.
// Automatically skips common VCS and cache directories (.git, node_modules, __pycache__, etc.).
func (e *Engine) IndexPaths(paths ...string) error {
	var wg sync.WaitGroup
	fileCh := make(chan string, 512)

	// Producer goroutine: discovers all files to be indexed
	wg.Add(1)
	go func() {
		defer wg.Done()
		seenDir := map[string]struct{}{} // Prevent duplicate directory processing
		for _, p := range paths {
			fi, err := os.Stat(p)
			if err != nil {
				continue
			}
			if fi.IsDir() {
				root := p
				if _, ok := seenDir[root]; ok {
					continue
				}
				seenDir[root] = struct{}{}
				// Recursively walk directory tree, skipping common ignore patterns
				filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
					if err != nil {
						return nil
					}
					if d.IsDir() {
						base := strings.ToLower(filepath.Base(path))
						// Skip VCS directories, Python caches, and package managers
						switch base {
						case ".git", ".hg", ".svn", "__pycache__", ".mypy_cache", ".pytest_cache", "venv", ".venv", "node_modules":
							return filepath.SkipDir
						}
						return nil
					}
					// Send file path to processing workers
					fileCh <- path
					return nil
				})
			} else {
				// Single file case
				fileCh <- p
			}
		}
		close(fileCh)
	}()

	// Consumer workers: parse files and extract symbols concurrently
	var cw sync.WaitGroup
	workers := 4 // Process 4 files simultaneously for optimal performance
	for range workers {
		cw.Add(1)
		go func() {
			defer cw.Done()
			for path := range fileCh {
				// Read file contents
				src, err := os.ReadFile(path)
				if err != nil {
					continue
				}
				// Find appropriate language adapter based on file extension
				adapter := e.pickAdapter(path)
				if adapter == nil {
					continue // Skip unsupported file types
				}
				// Parse source code into syntax tree using tree-sitter
				tree, err := adapter.Parse(path, src)
				if err != nil || tree == nil {
					continue // Skip files that failed to parse
				}
				// Extract symbols (defs, refs, imports) using language-specific queries
				fi, err := adapter.Extract(path, src, tree)
				if err != nil {
					continue
				}
				// Thread-safely merge file index into global project index
				e.Index.merge(fi)
			}
		}()
	}

	// Wait for both producer and all consumers to complete
	wg.Wait()
	cw.Wait()
	return nil
}

func (e *Engine) pickAdapter(path string) LanguageAdapter {
	for _, a := range e.Adapters {
		if a.CanHandle(path) {
			return a
		}
	}
	return nil
}

// FindDefinitionAt performs "go to definition" lookup for the symbol at the specified cursor position.
// Returns the definition location, candidate symbol IDs considered, and any error.
// The lookup process: 1) Find occurrence at cursor, 2) Resolve to symbol candidates, 3) Return first matching definition.
func (e *Engine) FindDefinitionAt(file string, line, col int) (DefLocation, []string, error) {
	// Normalize the file path to match how it's stored in the index
	normalizedFile := filepath.ToSlash(strings.TrimPrefix(file, "./"))

	// Get all symbol occurrences for this file from the index
	occs := e.GetFileOccurrences(normalizedFile)

	// Find the specific occurrence that contains the cursor position
	occ, ok := pickOccurrence(occs, line, col)
	if !ok {
		return DefLocation{}, nil, errors.New("no identifier at position")
	}

	// Get the language adapter for this file type
	adapter := e.pickAdapter(file)
	if adapter == nil {
		return DefLocation{}, nil, errors.New("no adapter for file")
	}

	// Let the language adapter resolve the occurrence to candidate symbol IDs
	src, _ := os.ReadFile(file)
	cands := adapter.ResolveAt(file, src, occ, e.Index)

	// Look up the first candidate that has a known definition in our index
	e.Index.mu.RLock()
	defer e.Index.mu.RUnlock()
	for _, sid := range cands {
		if def, ok := e.Index.Defs[sid]; ok {
			return def, cands, nil
		}
	}
	return DefLocation{}, cands, errors.New("definition not found")
}

func (e *Engine) FindReferences(symbolID string) ([]RefLocation, error) {
	e.Index.mu.RLock()
	defer e.Index.mu.RUnlock()
	refs := e.Index.Refs[symbolID]
	cp := make([]RefLocation, len(refs))
	copy(cp, refs)
	return cp, nil
}

func (e *Engine) GetDefinitions() map[string]DefLocation {
	e.Index.mu.RLock()
	defer e.Index.mu.RUnlock()
	out := make(map[string]DefLocation, len(e.Index.Defs))
	maps.Copy(out, e.Index.Defs)
	return out
}

// GetDefinitionTree returns a list of all definitions in the project, sorted by file.
func (e *Engine) GetDefinitionTree() []DefLocation {
	e.Index.mu.RLock()
	defer e.Index.mu.RUnlock()
	out := make([]DefLocation, 0, len(e.Index.Defs))
	for _, v := range e.Index.Defs {
		out = append(out, v)
	}
	// Sort by file name, then by def kind, then by def name
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func (e *Engine) GetFileOccurrences(file string) []Occurrence {
	e.Index.mu.RLock()
	defer e.Index.mu.RUnlock()
	cp := make([]Occurrence, len(e.Index.FileOcc[file]))
	copy(cp, e.Index.FileOcc[file])
	return cp
}

// pickOccurrence finds the symbol occurrence that contains the given cursor position.
// Used by FindDefinitionAt to identify which symbol the user is asking about.
// Returns the occurrence and true if found, or empty occurrence and false if not found.
func pickOccurrence(occs []Occurrence, line, col int) (Occurrence, bool) {
	pt := Pos{Line: line, Col: col}
	// Check each occurrence to see if the cursor position falls within its range
	for _, o := range occs {
		if beforeOrEq(o.Rng.Start, pt) && beforeOrEq(pt, o.Rng.End) {
			return o, true
		}
	}
	return Occurrence{}, false
}

func beforeOrEq(a, b Pos) bool {
	if a.Line < b.Line {
		return true
	}
	if a.Line > b.Line {
		return false
	}
	return a.Col <= b.Col
}

// symbolID generates a unique identifier for a symbol across the entire project.
// Format: "lang::file::name" or "lang::file::container.name" for methods.
// Used as the primary key for storing and looking up symbol definitions.
func symbolID(lang, file, container, name string) string {
	// Normalize file path for consistent symbol IDs across platforms
	file = filepath.ToSlash(file)
	file = strings.TrimPrefix(file, "./")

	if container != "" {
		// Method or nested symbol: include container (receiver type for Go methods)
		return lang + "::" + file + "::" + container + "." + name
	}
	// Top-level symbol: just language, file, and name
	return lang + "::" + file + "::" + name
}
