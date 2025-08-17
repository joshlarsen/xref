# xref

A high-performance cross-reference engine for multi-language code analysis and navigation.

## How It Works

This engine uses tree-sitter parsers to build a comprehensive symbol index across codebases. The core workflow involves three phases:

1. **Indexing Phase**: Recursively scans directories, parses files using tree-sitter grammars, and extracts symbol definitions and references using pre-defined queries
2. **Storage Phase**: Builds a unified project index mapping symbol IDs to their definitions and reference locations
3. **Query Phase**: Enables fast lookups to find definitions at cursor positions and locate all references to symbols

## Architecture

The system follows a plugin-based architecture with language adapters that implement:
- File type detection (`.go`, `.ts`, `.py`)
- Tree-sitter parsing for syntax trees
- Query execution using S-expressions to extract symbols
- Symbol resolution logic for "go to definition" functionality

## Core Components

- **Engine**: Main orchestrator that manages indexing and queries
- **ProjectIndex**: Thread-safe global symbol database with definitions, references, and name lookups
- **LanguageAdapter**: Plugin interface for adding new language support
- **Query System**: Uses tree-sitter query files (`.scm`) to extract symbols from ASTs

## Supported Languages

- **Go**: Functions, methods, types, variables, constants
- **TypeScript**: Functions, classes, interfaces, variables  
- **Python**: Functions, classes, variables
- **Ruby**: Basic symbol extraction

Each language adapter uses custom tree-sitter queries to identify language-specific constructs and build accurate symbol mappings.

## Definition and Reference Lookup Workflow

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                                INDEXING PHASE                                  │
└─────────────────────────────────────────────────────────────────────────────────┘

1. File Discovery
   ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
   │   hello.go  │    │   hello.ts  │    │   hello.py  │
   └─────────────┘    └─────────────┘    └─────────────┘
           │                  │                  │
           └──────────────────┼──────────────────┘
                              │
                              ▼
2. Language Detection & Parsing
   ┌─────────────────────────────────────────────────────────────┐
   │  LanguageAdapter.CanHandle() → Pick correct adapter        │
   │  LanguageAdapter.Parse() → Generate syntax tree            │
   └─────────────────────────────────────────────────────────────┘
                              │
                              ▼
3. Symbol Extraction (using .scm queries)
   ┌─────────────────────────────────────────────────────────────┐
   │  defs.scm  → Extract definitions (functions, types, vars)   │
   │  refs.scm  → Extract references (identifier usage)         │
   │  imports.scm → Extract import statements                    │
   └─────────────────────────────────────────────────────────────┘
                              │
                              ▼
4. Index Building
   ┌─────────────────────────────────────────────────────────────┐
   │                    ProjectIndex                             │
   │  ┌─────────────────────────────────────────────────────┐    │
   │  │ Defs: map[string]DefLocation                        │    │
   │  │   "go::hello.go::main" → {File: "hello.go", ...}    │    │
   │  │                                                     │    │
   │  │ NameLookup: map[string][]string                     │    │
   │  │   "go:main" → ["go::hello.go::main"]                │    │
   │  │                                                     │    │
   │  │ FileOcc: map[string][]Occurrence                    │    │
   │  │   "hello.go" → [{Name: "main", Range: ...}, ...]    │    │
   │  └─────────────────────────────────────────────────────┘    │
   └─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────────┐
│                             QUERY PHASE                                        │
└─────────────────────────────────────────────────────────────────────────────────┘

FindDefinitionAt(file, line, col):

1. Cursor Position → Symbol Occurrence
   ┌─────────────────────────────────────────────────────────────┐
   │  GetFileOccurrences(file) → []Occurrence                    │
   │  pickOccurrence(occs, line, col) → Find containing range    │
   └─────────────────────────────────────────────────────────────┘
                              │
                              ▼
2. Symbol Resolution
   ┌─────────────────────────────────────────────────────────────┐
   │  LanguageAdapter.ResolveAt(occurrence) → []candidateIDs     │
   │    • Try local file first (same-file definitions)          │
   │    • Fall back to global NameLookup                        │
   └─────────────────────────────────────────────────────────────┘
                              │
                              ▼
3. Definition Lookup
   ┌─────────────────────────────────────────────────────────────┐
   │  For each candidateID:                                      │
   │    if ProjectIndex.Defs[candidateID] exists:                │
   │      return DefLocation                                     │
   └─────────────────────────────────────────────────────────────┘

FindReferences(symbolID):

   ┌─────────────────────────────────────────────────────────────┐
   │  ProjectIndex.Refs[symbolID] → []RefLocation                │
   │  Return all known reference locations for this symbol       │
   └─────────────────────────────────────────────────────────────┘
```
