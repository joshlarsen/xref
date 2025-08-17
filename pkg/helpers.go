package xref

import (
	sitter "github.com/smacker/go-tree-sitter"
)

// execQuery executes a tree-sitter query against a syntax tree and calls a visitor function for each match.
// The visitor receives query captures and a function to resolve capture names by ID.
// This is the core mechanism for extracting symbols from parsed code.
func execQuery(src []byte, root *sitter.Node, q *sitter.Query, visit func([]sitter.QueryCapture, func(id uint32) string)) {
	cur := sitter.NewQueryCursor()
	defer cur.Close()
	
	// Execute the query against the syntax tree starting from root
	cur.Exec(q, root)
	
	// Process each match found by the query
	for {
		m, ok := cur.NextMatch()
		if !ok {
			break
		}
		// Call visitor with the captured nodes and name resolver
		visit(m.Captures, q.CaptureNameForId)
	}
}

// getByName extracts the text content of a named capture from tree-sitter query results.
// Searches through captures to find one matching the given name, then returns its source text.
// Returns empty string if the named capture is not found.
func getByName(src []byte, caps []sitter.QueryCapture, q *sitter.Query, name string) string {
	for _, c := range caps {
		if q.CaptureNameForId(c.Index) == name {
			// Extract the source text span covered by this capture
			return string(src[c.Node.StartByte():c.Node.EndByte()])
		}
	}
	return ""
}

// rangeByName extracts the source location range of a named capture from tree-sitter query results.
// Converts tree-sitter's 0-based coordinates to 1-based line/column positions.
// Returns empty range if the named capture is not found.
func rangeByName(src []byte, caps []sitter.QueryCapture, q *sitter.Query, name string) Range {
	for _, c := range caps {
		if q.CaptureNameForId(c.Index) == name {
			// Get start and end points from the syntax tree node
			sb, eb := c.Node.StartPoint(), c.Node.EndPoint()
			// Convert from 0-based to 1-based coordinates for consistency
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
