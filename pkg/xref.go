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
