# Technical Stack

> Last Updated: 2025-08-25
> Version: 1.0.0

## Application Framework

- **Framework:** Go (Golang)
- **Version:** 1.24.3

## Database

- **Primary Database:** In-memory ProjectIndex with thread-safe symbol storage

## Parsing Engine

- **Framework:** github.com/smacker/go-tree-sitter
- **Query System:** S-expression query files (.scm) for symbol extraction

## Language Support

- **Go:** Tree-sitter parser with custom .scm queries
- **TypeScript:** Tree-sitter parser with import/function extraction
- **Python:** Tree-sitter parser (roadmap priority)
- **Ruby:** Tree-sitter parser for future expansion

## Architecture

- **Concurrency:** Goroutine-based concurrent file processing
- **Plugin System:** Language adapter interface for extensibility
- **Storage:** Thread-safe in-memory symbol index
- **File System:** Efficient directory walking with concurrent indexing

## Build System

- **Package Manager:** Go modules
- **Build Tool:** Go toolchain
- **Testing:** Go testing framework

## Development Tools

- **Demo Application:** Standalone CLI for testing and demonstration
- **Query Files:** .scm files for customizable symbol extraction patterns