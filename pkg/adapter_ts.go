package xref

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	tsquery "github.com/smacker/go-tree-sitter/query"
	"github.com/smacker/go-tree-sitter/typescript"
)

type tsAdapter struct {
	qDefs, qRefs, qImport *tsquery.Query
}

func newTsAdapter() (LanguageAdapter, error) {
	tsLang := typescript.GetLanguage()
	qd, err := loadQuery("ts", "defs.scm", tsLang)
	if err != nil {
		return nil, err
	}
	qr, err := loadQuery("ts", "refs.scm", tsLang)
	if err != nil {
		return nil, err
	}
	qi, err := loadQuery("ts", "imports.scm", tsLang)
	if err != nil {
		return nil, err
	}
	return &tsAdapter{qDefs: qd, qRefs: qr, qImport: qi}, nil
}
func (t *tsAdapter) Lang() string { return "ts" }
func (t *tsAdapter) CanHandle(path string) bool {
	l := strings.ToLower(path)
	return strings.HasSuffix(l, ".ts") || strings.HasSuffix(l, ".mts") || strings.HasSuffix(l, ".cts")
}

func (t *tsAdapter) Parse(_ string, src []byte) (*sitter.Tree, error) {
	p := sitter.NewParser()
	p.SetLanguage(typescript.GetLanguage())
	return p.ParseCtx(nil, src)
}

func (t *tsAdapter) Extract(path string, src []byte, tree *sitter.Tree) (*FileIndex, error) {
	fi := &FileIndex{Lang: "ts", File: path, Defs: map[string]DefLocation{}, Refs: map[string][]RefLocation{}, Imports: map[string]string{}}
	root := tree.RootNode()
	execQuery(src, root, t.qImport, func(capts []tsquery.Capture, _ func(id uint32) string) {
		alias := getByName(src, capts, t.qImport, "alias")
		module := strings.Trim(getByName(src, capts, t.qImport, "module"), `"'`)
		rng := rangeByName(src, capts, t.qImport, "rng")
		if alias != "" && module != "" {
			fi.Imports[alias] = module
			fi.Occurrences = append(fi.Occurrences, Occurrence{Name: alias, KindHint: "import", Rng: rng})
		}
	})
	execQuery(src, root, t.qDefs, func(capts []tsquery.Capture, _ func(id uint32) string) {
		var name, kind string
		switch {
		case getByName(src, capts, t.qDefs, "fname") != "":
			name, kind = getByName(src, capts, t.qDefs, "fname"), "func"
		case getByName(src, capts, t.qDefs, "cname") != "":
			name, kind = getByName(src, capts, t.qDefs, "cname"), "class"
		case getByName(src, capts, t.qDefs, "iname") != "":
			name, kind = getByName(src, capts, t.qDefs, "iname"), "interface"
		case getByName(src, capts, t.qDefs, "ename") != "":
			name, kind = getByName(src, capts, t.qDefs, "ename"), "enum"
		default:
			name, kind = getByName(src, capts, t.qDefs, "vname"), "var"
		}
		if name == "" {
			return
		}
		rng := rangeByName(src, capts, t.qDefs, "rng")
		sid := symbolID("ts", path, "", name)
		fi.Defs[sid] = DefLocation{Lang: "ts", File: path, Rng: rng, Name: name, Kind: kind}
		fi.Occurrences = append(fi.Occurrences, Occurrence{Name: name, KindHint: "def", Rng: rng, SymbolID: sid})
	})
	execQuery(src, root, t.qRefs, func(capts []tsquery.Capture, _ func(id uint32) string) {
		id := getByName(src, capts, t.qRefs, "id")
		rng := rangeByName(src, capts, t.qRefs, "rng")
		if id != "" {
			fi.Occurrences = append(fi.Occurrences, Occurrence{Name: id, KindHint: "ref", Rng: rng})
		}
	})
	return fi, nil
}

func (t *tsAdapter) ResolveAt(path string, src []byte, occ Occurrence, pi *ProjectIndex) []string {
	pi.mu.RLock()
	defer pi.mu.RUnlock()
	for sid, d := range pi.Defs {
		if d.Lang == "ts" && d.File == path && d.Name == occ.Name {
			return []string{sid}
		}
	}
	return append([]string(nil), pi.NameLookup["ts:"+occ.Name]...)
}
