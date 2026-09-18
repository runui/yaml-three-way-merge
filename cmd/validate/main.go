package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/IceWhaleTech/ZimaOS-AppManagement/service/compose_app/pack/validationharness/internal/corpus"
	"github.com/IceWhaleTech/ZimaOS-AppManagement/service/compose_app/pack/validationharness/internal/validation"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("fixtures", "fixtures", "fixture directory")
	jsonOutput := flags.Bool("json", false, "write JSON summary")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	cases, _, err := corpus.Load(*root)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load fixtures: %v\n", err)
		return 2
	}
	results := make([]validation.Result, 0, len(cases))
	for _, item := range cases {
		results = append(results, validation.Run(item))
	}
	if err := validation.WriteSummary(stdout, results, *jsonOutput); err != nil {
		_, _ = fmt.Fprintf(stderr, "write report: %v\n", err)
		return 2
	}
	summary := validation.Summarize(results)
	if summary.Mismatched > 0 || summary.Errors > 0 || summary.NonIdempotent > 0 {
		return 1
	}
	return 0
}
