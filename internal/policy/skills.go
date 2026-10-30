package policy

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Price is what a skill costs to obtain. It is priced per credential, not per
// request: one token buys one skill for as long as the token is valid, which is
// the difference between pricing identities and pricing load (issue #1's
// "liquidity anchor" was the former, and its per-second arithmetic assumed the
// latter).
type Price struct {
	Amount   float64 `yaml:"amount" json:"amount"`
	Currency string  `yaml:"currency" json:"currency"`
	Unit     string  `yaml:"unit" json:"unit"`
}

// GateDecl says whether the surface enforces the token. "enforced" is a claim
// the checker verifies against a live surface (`check --probe --gate`); a claim
// it has not verified is reported, not trusted.
type GateDecl struct {
	Enforcement string `yaml:"enforcement" json:"enforcement"`
}

// SkillDecl is one priced skill: the scope a caller requests, the tool it
// unlocks, and what the token costs. The tool must be declared by a safety data
// sheet — that join is what the checker verifies.
type SkillDecl struct {
	Scope string   `yaml:"scope" json:"scope"`
	Tool  string   `yaml:"tool" json:"tool"`
	Note  string   `yaml:"note,omitempty" json:"note,omitempty"`
	Price Price    `yaml:"price" json:"price"`
	Gate  GateDecl `yaml:"gate,omitempty" json:"gate,omitempty"`
}

// Grants is the token-issuance policy. It is a commercial declaration, so it
// does not belong in a safety data sheet (which describes what a surface
// serves, not what it sells).
type Grants struct {
	Provider  string  `yaml:"provider" json:"provider"`
	Required  bool    `yaml:"required" json:"required"`
	UnitValue float64 `yaml:"unit_value" json:"unit_value"`
	Currency  string  `yaml:"currency" json:"currency"`

	// GrantTool is the tool that issues a token, and TokenArg/TokenField are
	// how a caller passes it and reads it back. Together they are what
	// `check --gate` drives: no token → refused, granted token → accepted.
	GrantTool  string `yaml:"grant_tool,omitempty" json:"grant_tool,omitempty"`
	TokenArg   string `yaml:"token_arg,omitempty" json:"token_arg,omitempty"`
	TokenField string `yaml:"token_field,omitempty" json:"token_field,omitempty"`
}

// Timing is the barista prototype's "temporal cloak" (issue #1): a client-side
// beat the server is supposed to police. It is carried here so the manifest can
// state it, not so a surface can rely on it.
type Timing struct {
	StandardizedLatencyMS int `yaml:"standardized_latency_ms" json:"standardized_latency_ms"`
	JitterMS              int `yaml:"jitter_ms" json:"jitter_ms"`
}

// Policy is the authored half of the manifest: the issuer's endpoints, the
// token price, and the skills for sale.
type Policy struct {
	Issuer                string `yaml:"issuer"`
	AuthorizationEndpoint string `yaml:"authorization_endpoint"`
	TokenEndpoint         string `yaml:"token_endpoint"`
	RegistrationEndpoint  string `yaml:"registration_endpoint"`

	Grants Grants      `yaml:"grants"`
	Timing *Timing     `yaml:"timing,omitempty"`
	Skills []SkillDecl `yaml:"skills"`
}

// LoadPolicy reads the policy YAML. Every field is optional: a repo with no
// policy file still gets a manifest describing its sheets.
func LoadPolicy(path string) (*Policy, error) {
	p := &Policy{}
	if path == "" {
		return p, nil
	}
	data, err := os.ReadFile(path) //nolint:gosec // G304: the policy path is the operator's own declaration
	if err != nil {
		return nil, fmt.Errorf("read policy: %w", err)
	}
	if err := yaml.Unmarshal(data, p); err != nil {
		return nil, fmt.Errorf("parse policy: %w", err)
	}
	if err := p.validate(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Policy) validate() error {
	if err := p.validateEndpoints(); err != nil {
		return err
	}

	seen := map[string]bool{}
	for i, s := range p.Skills {
		if s.Scope == "" {
			return fmt.Errorf("skill %d: scope is required", i+1)
		}
		if s.Tool == "" {
			return fmt.Errorf("skill %q: tool is required", s.Scope)
		}
		if seen[s.Scope] {
			return fmt.Errorf("skill %q is declared twice", s.Scope)
		}
		seen[s.Scope] = true
		if s.Price.Amount < 0 {
			return fmt.Errorf("skill %q: price cannot be negative", s.Scope)
		}
		switch s.Gate.Enforcement {
		case "", Declared, Enforced:
		default:
			return fmt.Errorf("skill %q: gate.enforcement must be %q or %q, got %q",
				s.Scope, Declared, Enforced, s.Gate.Enforcement)
		}
	}
	return nil
}

// validateEndpoints requires every endpoint the policy publishes to be an
// absolute http(s) URL with a host. A placeholder is a URL that will not
// resolve; it is not a URL that cannot be parsed, and a value like
// "issuer.example/v1/auth" or "https:///v1/auth" is neither a placeholder nor a
// reachable endpoint — it is a typo that the manifest would publish as fact.
func (p *Policy) validateEndpoints() error {
	for _, endpoint := range []struct{ field, raw string }{
		{"issuer", p.Issuer},
		{"authorization_endpoint", p.AuthorizationEndpoint},
		{"token_endpoint", p.TokenEndpoint},
		{"registration_endpoint", p.RegistrationEndpoint},
	} {
		if endpoint.raw == "" {
			continue
		}
		u, err := url.Parse(endpoint.raw)
		if err != nil {
			return fmt.Errorf("%s %q: %w", endpoint.field, endpoint.raw, err)
		}
		if u.Scheme != "https" && u.Scheme != "http" {
			return fmt.Errorf("%s %q: scheme must be http or https, got %q",
				endpoint.field, endpoint.raw, u.Scheme)
		}
		if u.Hostname() == "" {
			return fmt.Errorf("%s %q: no host", endpoint.field, endpoint.raw)
		}
	}
	return nil
}

// isPlaceholderHost reports whether a host is a reserved name that cannot
// belong to a live issuer: RFC 2606 (example.com/net/org, .example, .invalid,
// .test) and RFC 6761 (localhost). `.go` is not one of these — lezz.go is a
// repository name, not a registrable domain — which is why a policy can only be
// required to parse and to admit when it is living on reserved ground.
func isPlaceholderHost(host string) bool {
	h := strings.ToLower(strings.TrimSuffix(host, "."))
	if h == "" {
		return false
	}
	if h == "localhost" || strings.HasSuffix(h, ".localhost") {
		return true
	}
	switch h {
	case "example", "example.com", "example.net", "example.org":
		return true
	}
	for _, suffix := range []string{".example", ".invalid", ".test", ".example.com", ".example.net", ".example.org"} {
		if strings.HasSuffix(h, suffix) {
			return true
		}
	}
	return false
}
