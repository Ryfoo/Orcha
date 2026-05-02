package parser

import "fmt"

// compatibility encodes the output_type -> input_type matrix from the spec.
//
//	            text  json  filepath  list
//	text         OK    -     OK        -
//	json         OK    OK    -         -
//	filepath     OK    -     OK        -
//	list         OK    -     -         OK
var compatibility = map[OutputType]map[OutputType]bool{
	TypeText:     {TypeText: true, TypeFilepath: true},
	TypeJSON:     {TypeText: true, TypeJSON: true},
	TypeFilepath: {TypeText: true, TypeFilepath: true},
	TypeList:     {TypeText: true, TypeList: true},
}

// Compatible reports whether a step that produces `out` can feed into one that
// expects `in`.
func Compatible(out, in OutputType) bool {
	row, ok := compatibility[out]
	if !ok {
		return false
	}
	return row[in]
}

// TypeCheckPipeline walks each step and verifies that the previous step's
// output_type is compatible with the current step's expected input. The first
// step is exempt because its input comes from the user's call.
func TypeCheckPipeline(p *Pipeline, tasks map[string]*Task) error {
	for i := 1; i < len(p.Steps); i++ {
		prev := tasks[p.Steps[i-1].Task]
		curr := tasks[p.Steps[i].Task]
		want := curr.InputType()
		if !Compatible(prev.OutputType, want) {
			return fmt.Errorf(
				"pipeline %q: type mismatch at step %d: task %q outputs %q but task %q expects %q input",
				p.Name, i, prev.Name, prev.OutputType, curr.Name, want,
			)
		}
	}
	return nil
}
