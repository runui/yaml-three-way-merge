package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/IceWhaleTech/ZimaOS-AppManagement/service/compose_app/pack/validationharness/internal/validation"
)

func main() {
	jsonOutput := flag.Bool("json", false, "write the complete result as JSON")
	strict := flag.Bool("strict", false, "exit unsuccessfully when current behavior differs from desired semantics")
	flag.Parse()

	results := validation.RunAll(validation.Cases())
	var err error
	if *jsonOutput {
		err = validation.WriteJSON(os.Stdout, results)
	} else {
		err = validation.WriteMarkdown(os.Stdout, results)
	}
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "write report: %v\n", err)
		os.Exit(1)
	}
	if validation.HasClassificationMismatch(results) {
		os.Exit(1)
	}
	if *strict && validation.HasDesiredMismatch(results) {
		os.Exit(2)
	}
}
