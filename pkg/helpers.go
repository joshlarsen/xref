package xref

import (
	sitter "github.com/smacker/go-tree-sitter"
)

func execQuery(src []byte, root *sitter.Node, q *sitter.Query, visit func([]sitter.QueryCapture, func(id uint32) string)) {
	cur := sitter.NewQueryCursor()
	defer cur.Close()
	cur.Exec(q, root)
	for {
		m, ok := cur.NextMatch()
		if !ok {
			break
		}
		visit(m.Captures, q.CaptureNameForId)
	}
}

func getByName(src []byte, caps []sitter.QueryCapture, q *sitter.Query, name string) string {
	for _, c := range caps {
		if q.CaptureNameForId(c.Index) == name {
			return string(src[c.Node.StartByte():c.Node.EndByte()])
		}
	}
	return ""
}

func rangeByName(src []byte, caps []sitter.QueryCapture, q *sitter.Query, name string) Range {
	for _, c := range caps {
		if q.CaptureNameForId(c.Index) == name {
			sb, eb := c.Node.StartPoint(), c.Node.EndPoint()
			return Range{Start: Pos{int(sb.Row) + 1, int(sb.Column) + 1}, End: Pos{int(eb.Row) + 1, int(eb.Column) + 1}}
		}
	}
	return Range{}
}

func firstNonEmptyBy(src []byte, caps []sitter.QueryCapture, q *sitter.Query, names ...string) string {
	for _, n := range names {
		if v := getByName(src, caps, q, n); v != "" {
			return v
		}
	}
	return ""
}
