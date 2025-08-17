package xref

import (
	"embed"
	"fmt"
	"path/filepath"

	sitter "github.com/smacker/go-tree-sitter"
)

//go:embed queries/*/*.scm
var qfs embed.FS

func loadQuery(langFolder, file string, tsLang *sitter.Language) (*sitter.Query, error) {
	path := filepath.ToSlash(filepath.Join("queries", langFolder, file))
	b, err := qfs.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	q, err := sitter.NewQuery([]byte(b), tsLang)
	if err != nil {
		return nil, fmt.Errorf("compile %s: %w", path, err)
	}
	return q, nil
}
