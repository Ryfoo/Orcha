package runners

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ryfoo/orcha/internal/engine"
	"github.com/ryfoo/orcha/internal/parser"
)

var httpClient = &http.Client{Timeout: 60 * time.Second}

// RunHTTP performs the HTTP request described by the task. Headers, URL, and
// body all run through Interpolate so callers can inject inputs and env vars.
func RunHTTP(t *parser.Task, in engine.Value) (engine.Value, error) {
	url, err := engine.Interpolate(t.URL, in)
	if err != nil {
		return engine.Value{}, fmt.Errorf("interpolate url: %w", err)
	}

	var bodyReader io.Reader
	if t.Body != "" {
		body, err := engine.Interpolate(t.Body, in)
		if err != nil {
			return engine.Value{}, fmt.Errorf("interpolate body: %w", err)
		}
		bodyReader = strings.NewReader(body)
	}

	method := strings.ToUpper(t.Method)
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return engine.Value{}, fmt.Errorf("build request: %w", err)
	}
	for k, v := range t.Headers {
		hv, err := engine.Interpolate(v, in)
		if err != nil {
			return engine.Value{}, fmt.Errorf("interpolate header %s: %w", k, err)
		}
		req.Header.Set(k, hv)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return engine.Value{}, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return engine.Value{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return engine.Value{}, fmt.Errorf("http %d: %s", resp.StatusCode, truncateAt(string(body), 500))
	}

	return asOutput(string(body), t.OutputType)
}

func truncateAt(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
