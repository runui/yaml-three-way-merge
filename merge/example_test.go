package merge_test

import (
	"fmt"

	"github.com/runui/yaml-three-way-merge/merge"
)

func ExampleCompile() {
	scenario, err := merge.Compile(merge.Input{
		PreviousBase: []byte("services: {app: {image: old}}"),
		UserOverride: []byte("services: {app: {image: custom}}"),
		TargetBase:   []byte("services: {app: {image: new}}"),
	})
	if err != nil {
		panic(err)
	}
	plan, err := scenario.Analyze()
	if err != nil {
		panic(err)
	}
	result, err := plan.Apply()
	if err != nil {
		panic(err)
	}
	equal, err := merge.Equivalent(result.YAML, []byte("services: {app: {image: custom}}"))
	if err != nil {
		panic(err)
	}
	fmt.Println(equal, result.Report.UserReplayValid)
	// Output: true true
}
