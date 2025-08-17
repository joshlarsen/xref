package languages

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

// GetGoLanguage returns the actual tree-sitter Go grammar
func GetGoLanguage() *sitter.Language {
	return golang.GetLanguage()
}
