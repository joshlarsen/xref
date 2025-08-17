package main

import (
	"fmt"
	"log"

	"github.com/ghostsecurity/xref"
)

func main() {
	engine, err := xref.New() // default: Go + TS + Python adapters
	if err != nil {
		log.Fatal(err)
	}

	// Index a codebase (dir or specific files)
	if err := engine.IndexRoot("./some/project"); err != nil {
		log.Fatal(err)
	}

	// Find definition at a cursor
	def, cands, err := engine.FindDefinitionAt("./some/project/pkg/foo.go", 42, 17)
	if err != nil {
		fmt.Println("No definition:", err, "candidates:", cands)
	} else {
		fmt.Printf("Def: %s %s:%d:%d kind=%s\n", def.Name, def.File, def.Rng.Start.Line, def.Rng.Start.Col, def.Kind)
	}

	// List all known definitions (SymbolID -> Def)
	for sid, d := range engine.GetDefinitions() {
		fmt.Println(sid, "->", d.File, d.Kind, d.Name)
	}
}
