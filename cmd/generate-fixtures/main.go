package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/runui/yaml-three-way-merge/internal/corpus"
)

func main() {
	output := flag.String("output", "fixtures", "fixture output directory")
	flag.Parse()
	cases, manifest, err := corpus.Generate()
	if err == nil {
		err = corpus.Write(*output, cases, manifest)
	}
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "generate fixtures: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("generated %d cases for %d array fields in %s\n", len(cases), len(manifest.Fields), *output)
}
