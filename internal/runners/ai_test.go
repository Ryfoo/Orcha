package runners

import (
	"sync"
	"testing"

	"github.com/ryfoo/orcha/internal/engine"
	"github.com/ryfoo/orcha/internal/parser"
	providers "github.com/ryfoo/orcha/pkg"
)

// recorder is a tiny in-process Provider used only by these tests. It captures
// the last CompletionRequest the runner sent so we can assert on the
// system-message-omitted and default-model-fallback behavior.
type recorder struct {
	mu       sync.Mutex
	lastReq  providers.CompletionRequest
	defModel string
}

func (r *recorder) Name() string { return "recorder" }

func (r *recorder) DefaultModel() string { return r.defModel }

func (r *recorder) Complete(req providers.CompletionRequest) (providers.CompletionResponse, error) {
	r.mu.Lock()
	r.lastReq = req
	r.mu.Unlock()
	return providers.CompletionResponse{Content: "ok", Model: req.Model}, nil
}

var (
	recOnce sync.Once
	rec     = &recorder{defModel: "fallback-model"}
)

func registerRecorder(t *testing.T) *recorder {
	t.Helper()
	recOnce.Do(func() { providers.Register(rec) })
	return rec
}

func TestRunAIOmitsSystemWhenEmpty(t *testing.T) {
	r := registerRecorder(t)

	task := &parser.Task{
		Name:       "no-system",
		Type:       parser.KindAI,
		Provider:   "recorder",
		Model:      "explicit-model",
		Prompt:     "hello",
		OutputType: parser.TypeText,
	}
	if _, err := RunAI(task, engine.Value{Type: parser.TypeText, Raw: "x"}); err != nil {
		t.Fatal(err)
	}
	msgs := r.lastReq.Messages
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message (user only), got %d: %+v", len(msgs), msgs)
	}
	if msgs[0].Role != "user" {
		t.Errorf("only message should be user, got role=%q", msgs[0].Role)
	}
}

func TestRunAIIncludesSystemWhenSet(t *testing.T) {
	r := registerRecorder(t)

	task := &parser.Task{
		Name:       "with-system",
		Type:       parser.KindAI,
		Provider:   "recorder",
		Model:      "explicit-model",
		System:     "be brief",
		Prompt:     "hello",
		OutputType: parser.TypeText,
	}
	if _, err := RunAI(task, engine.Value{Type: parser.TypeText, Raw: "x"}); err != nil {
		t.Fatal(err)
	}
	msgs := r.lastReq.Messages
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "system" || msgs[0].Content != "be brief" {
		t.Errorf("system message wrong: %+v", msgs[0])
	}
}

func TestRunAIFallsBackToDefaultModel(t *testing.T) {
	r := registerRecorder(t)

	task := &parser.Task{
		Name:       "no-model",
		Type:       parser.KindAI,
		Provider:   "recorder",
		Prompt:     "hello",
		OutputType: parser.TypeText,
	}
	if _, err := RunAI(task, engine.Value{Type: parser.TypeText, Raw: "x"}); err != nil {
		t.Fatal(err)
	}
	if r.lastReq.Model != "fallback-model" {
		t.Errorf("model = %q, want fallback-model", r.lastReq.Model)
	}
}
