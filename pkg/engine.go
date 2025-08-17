package xref

import (
	"errors"
	"os"
	"path/filepath"
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

func (pi *ProjectIndex) merge(fi *FileIndex) {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	for sid, d := range fi.Defs {
		pi.Defs[sid] = d
		key := d.Lang + ":" + d.Name
		pi.NameLookup[key] = append(pi.NameLookup[key], sid)
	}
	for sid, refs := range fi.Refs {
		pi.Refs[sid] = append(pi.Refs[sid], refs...)
	}
	pi.FileOcc[fi.File] = append(pi.FileOcc[fi.File], fi.Occurrences...)
}

func New(adapters ...LanguageAdapter) (*Engine, error) {
	if len(adapters) == 0 {
		py, err := newPyAdapter()
		if err != nil {
			return nil, err
		}
		ts, err := newTsAdapter()
		if err != nil {
			return nil, err
		}
		goa, err := newGoAdapter()
		if err != nil {
			return nil, err
		}
		adapters = []LanguageAdapter{goa, ts, py}
	}
	return &Engine{Index: newProjectIndex(), Adapters: adapters}, nil
}

func (e *Engine) IndexRoot(root string) error {
	return e.IndexPaths(root)
}

func (e *Engine) IndexPaths(paths ...string) error {
	var wg sync.WaitGroup
	fileCh := make(chan string, 512)

	// produce file paths
	wg.Add(1)
	go func() {
		defer wg.Done()
		seenDir := map[string]struct{}{}
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
				filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
					if err != nil {
						return nil
					}
					if d.IsDir() {
						base := strings.ToLower(filepath.Base(path))
						switch base {
						case ".git", ".hg", ".svn", "__pycache__", ".mypy_cache", ".pytest_cache", "venv", ".venv", "node_modules":
							return filepath.SkipDir
						}
						return nil
					}
					fileCh <- path
					return nil
				})
			} else {
				fileCh <- p
			}
		}
		close(fileCh)
	}()

	// consume
	var cw sync.WaitGroup
	workers := 4
	for i := 0; i < workers; i++ {
		cw.Add(1)
		go func() {
			defer cw.Done()
			for path := range fileCh {
				src, err := os.ReadFile(path)
				if err != nil {
					continue
				}
				adapter := e.pickAdapter(path)
				if adapter == nil {
					continue
				}
				tree, err := adapter.Parse(path, src)
				if err != nil || tree == nil {
					continue
				}
				fi, err := adapter.Extract(path, src, tree)
				if err != nil {
					continue
				}
				e.Index.merge(fi)
			}
		}()
	}

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

func (e *Engine) FindDefinitionAt(file string, line, col int) (DefLocation, []string, error) {
	occs := e.GetFileOccurrences(file)
	occ, ok := pickOccurrence(occs, line, col)
	if !ok {
		return DefLocation{}, nil, errors.New("no identifier at position")
	}
	adapter := e.pickAdapter(file)
	if adapter == nil {
		return DefLocation{}, nil, errors.New("no adapter for file")
	}
	src, _ := os.ReadFile(file)
	cands := adapter.ResolveAt(file, src, occ, e.Index)
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
	for k, v := range e.Index.Defs {
		out[k] = v
	}
	return out
}

func (e *Engine) GetFileOccurrences(file string) []Occurrence {
	e.Index.mu.RLock()
	defer e.Index.mu.RUnlock()
	cp := make([]Occurrence, len(e.Index.FileOcc[file]))
	copy(cp, e.Index.FileOcc[file])
	return cp
}

func pickOccurrence(occs []Occurrence, line, col int) (Occurrence, bool) {
	pt := Pos{Line: line, Col: col}
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

func symbolID(lang, file, container, name string) string {
	file = filepath.ToSlash(file)
	file = strings.TrimPrefix(file, "./")
	if container != "" {
		return lang + "::" + file + "::" + container + "." + name
	}
	return lang + "::" + file + "::" + name
}
