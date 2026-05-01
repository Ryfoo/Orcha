package engine

import (
	"os"
	"testing"

	"github.com/ryfoo/orcha/internal/parser"
)

func TestInterpolateInputAndEnv(t *testing.T) {
	t.Setenv("FOO", "bar")
	v := Value{Type: parser.TypeText, Raw: "world"}
	got, err := Interpolate("hello {{$input}} from {{$env.FOO}}", v)
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello world from bar" {
		t.Fatalf("got %q", got)
	}
}

func TestInterpolateMissingEnvIsEmpty(t *testing.T) {
	os.Unsetenv("MISSING_VAR")
	v := Value{Type: parser.TypeText, Raw: "x"}
	got, _ := Interpolate("[{{$env.MISSING_VAR}}]{{$input}}", v)
	if got != "[]x" {
		t.Fatalf("got %q", got)
	}
}

func TestInterpolateJSONInputSerializes(t *testing.T) {
	v := Value{Type: parser.TypeJSON, Raw: map[string]any{"k": "v"}}
	got, err := Interpolate("data={{$input}}", v)
	if err != nil {
		t.Fatal(err)
	}
	if got != `data={"k":"v"}` {
		t.Fatalf("got %q", got)
	}
}

func TestInterpolateListJoinsByNewline(t *testing.T) {
	v := Value{Type: parser.TypeList, Raw: []string{"a", "b", "c"}}
	got, err := Interpolate("{{$input}}", v)
	if err != nil {
		t.Fatal(err)
	}
	if got != "a\nb\nc" {
		t.Fatalf("got %q", got)
	}
}
