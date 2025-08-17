package xref

import sitter "github.com/smacker/go-tree-sitter"

// Pos/Range/DefLocation/RefLocation are stable types you can print or JSON.
type (
	Pos   struct{ Line, Col int } // 1-based
	Range struct{ Start, End Pos }
)

type DefLocation struct {
	Lang string
	File string
	Rng  Range
	Name string
	Kind string // e.g., func, class, var, type, ...
}

type RefLocation struct {
	Lang string
	File string
	Rng  Range
}

// LanguageAdapter lets you add languages (tree-sitter-based).
type LanguageAdapter interface {
	Lang() string
	CanHandle(path string) bool
	Parse(path string, src []byte) (*sitter.Tree, error) // thin alias over ts parser
	Extract(path string, src []byte, tree *sitter.Tree) (*FileIndex, error)
	ResolveAt(path string, src []byte, occ Occurrence, pi *ProjectIndex) []string
}

// Engine holds the cross-file index and exposes queries.
type Engine struct {
	Index    *ProjectIndex
	Adapters []LanguageAdapter
}

// New creates an Engine. If no adapters are provided, it registers Go+TS+Py.
func New(adapters ...LanguageAdapter) (*Engine, error)

// IndexRoot recursively indexes a directory (skipping VCS/venv caches).
func (e *Engine) IndexRoot(root string) error

// IndexPaths indexes specific files or directories (recursive for dirs).
func (e *Engine) IndexPaths(paths ...string) error

// FindDefinitionAt returns the best definition for the identifier at file:line:col.
// Also returns the set of candidate SymbolIDs it considered (debugging).
func (e *Engine) FindDefinitionAt(file string, line, col int) (DefLocation, []string, error)

// FindReferences returns all reference locations for a SymbolID (if collected).
func (e *Engine) FindReferences(symbolID string) ([]RefLocation, error)

// GetDefinitions returns a copy of the definition map (SymbolID -> Def).
func (e *Engine) GetDefinitions() map[string]DefLocation

// GetFileOccurrences returns all raw occurrences we captured for a file (debugging).
func (e *Engine) GetFileOccurrences(file string) []Occurrence
