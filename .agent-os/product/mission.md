# Product Mission

> Last Updated: 2025-08-25
> Version: 1.0.0

## Pitch

xref is a minimal, focused cross-reference engine for parsing source code imports and functions across multiple languages using tree-sitter. We build a high-performance, accurate cross-reference engine that does one thing well - parse source code imports and functions using tree-sitter for multi-language code analysis and navigation.

## Users

- **Internal development team members** needing code analysis tools for understanding and navigating codebases
- **Open source contributors** working on code tooling projects who need reliable symbol extraction
- **Developers building IDE extensions or language servers** requiring accurate cross-reference capabilities

## The Problem

Existing code analysis tools are often bloated with features, slow to parse large codebases, or inaccurate in their symbol extraction. Developers need a focused, high-performance tool that reliably parses imports and function definitions across multiple programming languages without unnecessary complexity.

## Differentiators

- **Laser Focus**: Does one thing exceptionally well - parses imports and functions, nothing more
- **Tree-sitter Foundation**: Built on proven tree-sitter parsing technology for accuracy across languages
- **Performance First**: Concurrent processing architecture designed for speed
- **Language Agnostic**: Plugin system supports multiple programming languages through unified interface
- **Minimal Footprint**: No bloated features, just essential cross-reference functionality

## Key Features

- **Core Engine with ProjectIndex** for thread-safe symbol storage and retrieval
- **Language adapter plugin system** enabling support for multiple programming languages
- **Tree-sitter based parsing** with .scm query system for precise symbol extraction
- **Symbol resolution and "go to definition" functionality** for code navigation
- **Concurrent file indexing** with efficient directory walking
- **S-expression query files (.scm)** for customizable symbol extraction patterns
- **Demo application** showcasing basic usage and integration patterns