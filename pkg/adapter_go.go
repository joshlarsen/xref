package xref

import (
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	tgo "github.com/smacker/go-tree-sitter/golang"
	tsquery "github.com/smacker/go-tree-sitter/query"
)

type goAdapter struct {
	qDefs, qRefs, qImport *tsquery.Query
}

func newGoAdapter() (LanguageAdapter, error) {
	tsLang := tgo.GetLanguage()
	qd, err := loadQuery("go", "defs.scm", tsLang)
	if err != nil {
		return nil, err
	}
	qr, err := loadQuery("go", "refs.scm", tsLang)
	if err != nil {
		return nil, err
	}
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
	p.SetLanguage(tgo.GetLanguage())
	return p.ParseCtx(nil, src)
}

func (g *goAdapter) Extract(path string, src []byte, tree *sitter.Tree) (*FileIndex, error) {
	fi := &FileIndex{Lang: "go", File: path, Defs: map[string]DefLocation{}, Refs: map[string][]RefLocation{}, Imports: map[string]string{}}
	root := tree.RootNode()
	execQuery(src, root, g.qImport, func(capts []tsquery.Capture, _ func(id uint32) string) {
		alias := getByName(src, capts, g.qImport, "alias")
		ipath := strings.Trim(getByName(src, capts, g.qImport, "path"), "`\"")
		rng := rangeByName(src, capts, g.qImport, "rng")
		if ipath != "" {
			if alias == "" {
				alias = filepath.Base(ipath)
			}
			fi.Imports[alias] = ipath
			fi.Occurrences = append(fi.Occurrences, Occurrence{Name: alias, KindHint: "import", Rng: rng})
		}
	})
	execQuery(src, root, g.qDefs, func(capts []tsquery.Capture, _ func(id uint32) string) {
		var name, kind, recv string
		switch {
		case getByName(src, capts, g.qDefs, "fname") != "" && getByName(src, capts, g.qDefs, "mrecv") == "":
			name, kind = getByName(src, capts, g.qDefs, "fname"), "func"
		case getByName(src, capts, g.qDefs, "fname") != "" && getByName(src, capts, g.qDefs, "mrecv") != "":
			name, kind, recv = getByName(src, capts, g.qDefs, "fname"), "func", getByName(src, capts, g.qDefs, "mrecv")
			recv = strings.TrimPrefix(recv, "*")
		case getByName(src, capts, g.qDefs, "tname") != "":
			name, kind = getByName(src, capts, g.qDefs, "tname"), "type"
		case getByName(src, capts, g.qDefs, "vname") != "":
			name, kind = getByName(src, capts, g.qDefs, "vname"), "var"
		default:
			name, kind = getByName(src, capts, g.qDefs, "cname"), "const"
		}
		if name == "" {
			return
		}
		rng := rangeByName(src, capts, g.qDefs, "rng")
		sid := symbolID("go", path, recv, name)
		fi.Defs[sid] = DefLocation{Lang: "go", File: path, Rng: rng, Name: name, Kind: kind}
		fi.Occurrences = append(fi.Occurrences, Occurrence{Name: name, KindHint: "def", Rng: rng, SymbolID: sid})
	})
	execQuery(src, root, g.qRefs, func(capts []tsquery.Capture, _ func(id uint32) string) {
		id := getByName(src, capts, g.qRefs, "id")
		rng := rangeByName(src, capts, g.qRefs, "rng")
		if id != "" {
			fi.Occurrences = append(fi.Occurrences, Occurrence{Name: id, KindHint: "ref", Rng: rng})
		}
	})
	return fi, nil
}

func (g *goAdapter) ResolveAt(path string, _ []byte, occ Occurrence, pi *ProjectIndex) []string {
	pi.mu.RLock()
	defer pi.mu.RUnlock()
	for sid, d := range pi.Defs {
		if d.Lang == "go" && d.File == path && d.Name == occ.Name {
			return []string{sid}
		}
	}
	return append([]string(nil), pi.NameLookup["go:"+occ.Name]...)
}
