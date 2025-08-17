# xref

This is a cross-reference engine for code analysis that:

- Indexes codebases using tree-sitter parsers for multiple languages (Go, TypeScript, Python)
- Finds definitions of symbols at specific cursor positions
- Tracks references to symbols across files
- Extracts semantic information like function definitions, imports, and variable declarations
- Provides a unified API for code navigation across different programming languages

Key Features:

- Language Detection: Automatically detects .go and .ts files
- Parsing: Real tree-sitter parsers for Go, TypeScript, and Python
- Symbol Extraction: Extracts definitions (functions, classes, variables)
- Indexing: Creates searchable symbol database
- API: Clean Go API for code analysis tools
