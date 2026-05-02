package parser

import "testing"

func TestCompatibleMatrix(t *testing.T) {
	cases := []struct {
		out, in OutputType
		want    bool
	}{
		{TypeText, TypeText, true},
		{TypeText, TypeJSON, false},
		{TypeText, TypeFilepath, true},
		{TypeText, TypeList, false},
		{TypeJSON, TypeText, true},
		{TypeJSON, TypeJSON, true},
		{TypeJSON, TypeFilepath, false},
		{TypeJSON, TypeList, false},
		{TypeFilepath, TypeText, true},
		{TypeFilepath, TypeJSON, false},
		{TypeFilepath, TypeFilepath, true},
		{TypeFilepath, TypeList, false},
		{TypeList, TypeText, true},
		{TypeList, TypeJSON, false},
		{TypeList, TypeFilepath, false},
		{TypeList, TypeList, true},
	}
	for _, c := range cases {
		got := Compatible(c.out, c.in)
		if got != c.want {
			t.Errorf("Compatible(%s,%s) = %v, want %v", c.out, c.in, got, c.want)
		}
	}
}

func TestParseRejectsBadFlow(t *testing.T) {
	yaml := []byte(`
tasks:
  produce-text:
    type: file
    operation: read
    path: x.txt
    output_type: text
  expect-json:
    type: http
    method: POST
    url: http://example.com/
    output_type: json
pipelines:
  bad:
    steps:
      - task: produce-text
      - task: expect-json
`)
	// http expects text input, produce-text outputs text — this should
	// actually pass typecheck; we use an explicit list->filepath case instead.
	_, err := Parse(yaml)
	if err != nil {
		t.Fatalf("expected this fixture to parse cleanly, got %v", err)
	}

	bad := []byte(`
tasks:
  emit-list:
    type: file
    operation: read
    path: x.txt
    output_type: list
  read-then:
    type: file
    operation: read
    path: "{{$input}}"
    output_type: text
pipelines:
  bad:
    steps:
      - task: emit-list
      - task: read-then
`)
	if _, err := Parse(bad); err == nil {
		t.Fatal("expected list->filepath typecheck failure, got nil")
	}
}

func TestParseRejectsMissingPrompt(t *testing.T) {
	// model and system are now optional (defaulted); only prompt is required.
	yaml := []byte(`
tasks:
  ai-incomplete:
    type: ai
    provider: openai
    output_type: text
`)
	if _, err := Parse(yaml); err == nil {
		t.Fatal("expected validation error for missing prompt")
	}
}
