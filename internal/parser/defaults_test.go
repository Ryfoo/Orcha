package parser

import "testing"

func TestMinimalAITaskParses(t *testing.T) {
	doc, err := Parse([]byte(`
tasks:
  hello:
    type: ai
    provider: openai
    prompt: "Hi {{$input}}"
`))
	if err != nil {
		t.Fatalf("expected minimal AI task to parse, got: %v", err)
	}
	got := doc.Tasks["hello"]
	if got.OutputType != TypeText {
		t.Errorf("output_type default = %q, want text", got.OutputType)
	}
	if got.Model != "" {
		t.Errorf("model should remain empty (deferred to runner), got %q", got.Model)
	}
	if got.System != "" {
		t.Errorf("system should remain empty, got %q", got.System)
	}
}

func TestMinimalHTTPTaskParses(t *testing.T) {
	doc, err := Parse([]byte(`
tasks:
  fetch:
    type: http
    url: "https://example.com"
`))
	if err != nil {
		t.Fatalf("expected minimal HTTP task to parse, got: %v", err)
	}
	got := doc.Tasks["fetch"]
	if got.Method != "GET" {
		t.Errorf("method default = %q, want GET", got.Method)
	}
	if got.OutputType != TypeText {
		t.Errorf("output_type default = %q, want text", got.OutputType)
	}
}

func TestMinimalFileTaskParses(t *testing.T) {
	doc, err := Parse([]byte(`
tasks:
  load:
    type: file
    path: "./x.txt"
`))
	if err != nil {
		t.Fatalf("expected minimal file task to parse, got: %v", err)
	}
	got := doc.Tasks["load"]
	if got.Operation != OpRead {
		t.Errorf("operation default = %q, want read", got.Operation)
	}
	if got.OutputType != TypeText {
		t.Errorf("output_type default = %q, want text", got.OutputType)
	}
}

func TestFileWriteOutputTypeDefaultsToFilepath(t *testing.T) {
	doc, err := Parse([]byte(`
tasks:
  save:
    type: file
    operation: write
    path: "./out.txt"
    content: "hello"
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := doc.Tasks["save"].OutputType; got != TypeFilepath {
		t.Errorf("write default output_type = %q, want filepath", got)
	}
}

func TestAIPromptStillRequired(t *testing.T) {
	_, err := Parse([]byte(`
tasks:
  bad:
    type: ai
    provider: openai
`))
	if err == nil {
		t.Fatal("expected error for AI task without prompt")
	}
}

func TestHTTPRejectsBadMethod(t *testing.T) {
	_, err := Parse([]byte(`
tasks:
  bad:
    type: http
    method: TELEPORT
    url: "https://example.com"
`))
	if err == nil {
		t.Fatal("expected error for unsupported http method")
	}
}
