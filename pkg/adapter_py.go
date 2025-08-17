package xref

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"
)

type pyAdapter struct {
	qDefs, qRefs, qImport *sitter.Query
}

func newPyAdapter() (LanguageAdapter, error) {
	tsLang := python.GetLanguage()
	qd, err := loadQuery("py", "defs.scm", tsLang)
	if err != nil {
		return nil, err
	}
	qr, err := loadQuery("py", "refs.scm", tsLang)
	if err != nil {
		return nil, err
	}
	qi, err := loadQuery("py", "imports.scm", tsLang)
	if err != nil {
		return nil, err
	}
	return &pyAdapter{qDefs: qd, qRefs: qr, qImport: qi}, nil
}

func (p *pyAdapter) Lang() string { return "py" }
func (p *pyAdapter) CanHandle(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".py")
}

func (p *pyAdapter) Parse(_ string, src []byte) (*sitter.Tree, error) {
	parser := sitter.NewParser()
	lang := python.GetLanguage()
	if lang == nil {
		return nil, nil // Language not available
	}
	parser.SetLanguage(lang)
	return parser.ParseCtx(context.Background(), nil, src)
}

func (p *pyAdapter) Extract(path string, src []byte, tree *sitter.Tree) (*FileIndex, error) {
	fi := &FileIndex{
		Lang: "py", File: path,
		Defs: map[string]DefLocation{}, Refs: map[string][]RefLocation{},
		Imports: map[string]string{},
	}
	if tree == nil {
		return fi, nil // Return empty index if parsing failed
	}
	root := tree.RootNode()
	execQuery(src, root, p.qImport, func(capts []sitter.QueryCapture, get func(id uint32) string) {
		mod := getByName(src, capts, p.qImport, "module")
		alias := getByName(src, capts, p.qImport, "alias")
		rng := rangeByName(src, capts, p.qImport, "m_rng")
		if alias == "" && mod != "" {
			alias = strings.Split(mod, ".")[0]
		}
		if alias != "" {
			fi.Imports[alias] = mod
			fi.Occurrences = append(fi.Occurrences, Occurrence{Name: alias, KindHint: "import", Rng: rng})
		}
	})
	execQuery(src, root, p.qDefs, func(capts []sitter.QueryCapture, _ func(id uint32) string) {
		name, kind := firstNonEmptyBy(src, capts, p.qDefs, "fname", "cname", "aname"), ""
		switch {
		case getByName(src, capts, p.qDefs, "fname") != "":
			kind = "func"
		case getByName(src, capts, p.qDefs, "cname") != "":
			kind = "class"
		default:
			kind = "var"
		}
		rng := rangeByName(src, capts, p.qDefs, "rng")
		if name == "" {
			return
		}
		sid := symbolID("py", path, "", name)
		fi.Defs[sid] = DefLocation{Lang: "py", File: path, Rng: rng, Name: name, Kind: kind}
		fi.Occurrences = append(fi.Occurrences, Occurrence{Name: name, KindHint: "def", Rng: rng, SymbolID: sid})
	})
	execQuery(src, root, p.qRefs, func(capts []sitter.QueryCapture, _ func(id uint32) string) {
		id := getByName(src, capts, p.qRefs, "id")
		rng := rangeByName(src, capts, p.qRefs, "rng")
		if id != "" {
			fi.Occurrences = append(fi.Occurrences, Occurrence{Name: id, KindHint: "ref", Rng: rng})
		}
	})
	return fi, nil
}

func (p *pyAdapter) ResolveAt(path string, src []byte, occ Occurrence, pi *ProjectIndex) []string {
	pi.mu.RLock()
	defer pi.mu.RUnlock()
	for sid, d := range pi.Defs {
		if d.Lang == "py" && d.File == path && d.Name == occ.Name {
			return []string{sid}
		}
	}
	key := "py:" + occ.Name
	return append([]string(nil), pi.NameLookup[key]...)
}
