package providers

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

var (
	regMu     sync.RWMutex
	providers = map[string]Provider{}
)

// Register adds a provider to the global registry. Intended to be called from
// init() blocks. Panics on duplicate registration so misconfigurations surface
// at startup rather than as silent overrides.
func Register(p Provider) {
	if p == nil {
		panic("providers: Register called with nil provider")
	}
	name := p.Name()
	if name == "" {
		panic("providers: provider returned empty Name()")
	}
	regMu.Lock()
	defer regMu.Unlock()
	if _, exists := providers[name]; exists {
		panic(fmt.Sprintf("providers: duplicate registration for %q", name))
	}
	providers[name] = p
}

// Get returns the provider registered under name, or an error listing the
// available providers when the name is unknown.
func Get(name string) (Provider, error) {
	regMu.RLock()
	defer regMu.RUnlock()
	p, ok := providers[name]
	if !ok {
		known := make([]string, 0, len(providers))
		for k := range providers {
			known = append(known, k)
		}
		return nil, fmt.Errorf("unknown provider %q (registered: %s)", name, strings.Join(known, ", "))
	}
	return p, nil
}

// Names returns all registered provider names. Useful for diagnostics.
func Names() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]string, 0, len(providers))
	for k := range providers {
		out = append(out, k)
	}
	return out
}

// ResolveAPIKey looks up the API key for a provider by convention:
//
//	openai     -> OPENAI_API_KEY
//	anthropic  -> ANTHROPIC_API_KEY
//	<other>    -> ORCHA_<NAME>_API_KEY (uppercased, hyphens to underscores)
//
// Returns "" if no env var is set; callers decide whether the provider needs
// a key.
func ResolveAPIKey(providerName string) string {
	upper := strings.ToUpper(strings.ReplaceAll(providerName, "-", "_"))
	switch upper {
	case "OPENAI":
		return os.Getenv("OPENAI_API_KEY")
	case "ANTHROPIC":
		return os.Getenv("ANTHROPIC_API_KEY")
	default:
		return os.Getenv("ORCHA_" + upper + "_API_KEY")
	}
}
