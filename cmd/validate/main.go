package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/IceWhaleTech/ZimaOS-AppManagement/service/compose_app/pack/validationharness/internal/corpus"
	"github.com/IceWhaleTech/ZimaOS-AppManagement/service/compose_app/pack/validationharness/internal/validation"
)

func main() {
	root := flag.String("fixtures", "fixtures", "fixture directory")
	jsonOutput := flag.Bool("json", false, "write JSON summary")
	flag.Parse()
	cases, _, err := corpus.Load(*root)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "load fixtures: %v\n", err)
		os.Exit(2)
	}
	results := make([]validation.Result, 0, len(cases))
	for _, item := range cases {
		results = append(results, validation.Run(item))
	}
	if err := validation.WriteSummary(os.Stdout, results, *jsonOutput); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "write report: %v\n", err)
		os.Exit(2)
	}
	summary := validation.Summarize(results)
	if summary.Mismatched > 0 || summary.Errors > 0 || summary.NonIdempotent > 0 {
		os.Exit(1)
	}
}
