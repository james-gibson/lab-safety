package policy

import (
	"fmt"
	"os"

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

// SkillDecl is one priced skill: the scope a caller requests, the tool it
// unlocks, and what the token costs. The tool must be declared by a safety data
// sheet — that join is what the checker verifies.
type SkillDecl struct {
	Scope string `yaml:"scope" json:"scope"`
	Tool  string `yaml:"tool" json:"tool"`
	Note  string `yaml:"note,omitempty" json:"note,omitempty"`
	Price Price  `yaml:"price" json:"price"`
}

// Grants is the token-issuance policy. It is a commercial declaration, so it
// does not belong in a safety data sheet (which describes what a surface
// serves, not what it sells).
type Grants struct {
	Provider  string  `yaml:"provider" json:"provider"`
	Required  bool    `yaml:"required" json:"required"`
	UnitValue float64 `yaml:"unit_value" json:"unit_value"`
	Currency  string  `yaml:"currency" json:"currency"`
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
	}
	return nil
}
