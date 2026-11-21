package ingress

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
)

// Agreement is an operator-authored mapping. It is the only source of a
// cluster address and namespace; nothing in an inbound request can supply one.
type Agreement struct {
	ID        string   `json:"id"`
	Namespace string   `json:"namespace"`
	Border    string   `json:"border"`
	Aliases   []string `json:"aliases"`
	Rung      int      `json:"rung,omitempty"`
}

type agreementsFile struct {
	Agreements []Agreement `json:"agreements"`
}

// Route is what the resolver returns for an accepted name. Every field is read
// from an Agreement, never synthesised from the input.
type Route struct {
	Address     string `json:"address"`
	Namespace   string `json:"namespace"`
	AgreementID string `json:"agreement_id"`
	Rung        int    `json:"rung"`
}

// Decision is the typed outcome of resolution: a route, or a quarantine reason.
type Decision struct {
	Decision       string `json:"decision"`
	Reason         string `json:"reason,omitempty"`
	Route          *Route `json:"route,omitempty"`
	UpstreamStatus int    `json:"upstream_status,omitempty"`
}

// Resolver indexes agreements by canonical alias.
type Resolver struct {
	agreements []Agreement
	byAlias    map[string][]int
}

// LoadAgreements reads the agreed mappings. A malformed declaration is a loud
// startup error, not a resolver that quietly guesses.
func LoadAgreements(r io.Reader) (*Resolver, error) {
	var file agreementsFile
	if err := json.NewDecoder(io.LimitReader(r, 1<<20)).Decode(&file); err != nil {
		return nil, fmt.Errorf("decode agreements: %w", err)
	}

	res := &Resolver{agreements: file.Agreements, byAlias: make(map[string][]int)}
	for i, a := range file.Agreements {
		if a.ID == "" || a.Namespace == "" || a.Border == "" || len(a.Aliases) == 0 {
			return nil, fmt.Errorf("agreement %d: id, namespace, border and aliases are required", i)
		}
		u, err := url.Parse(a.Border)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, fmt.Errorf("agreement %q: border must be an http(s) URL", a.ID)
		}
		for _, alias := range a.Aliases {
			name, err := ParseName(alias)
			if err != nil {
				return nil, fmt.Errorf("agreement %q: alias %q is not a valid name", a.ID, alias)
			}
			if !hasIndex(res.byAlias[name], i) {
				res.byAlias[name] = append(res.byAlias[name], i)
			}
		}
	}
	return res, nil
}

// NewEmptyResolver is the fail-closed default: every name is unknown.
func NewEmptyResolver() *Resolver {
	return &Resolver{byAlias: make(map[string][]int)}
}

// Resolve is total. It returns a route from an agreement, or a typed refusal:
// malformed, unknown, or ambiguous. It never picks "the only one".
func (r *Resolver) Resolve(raw string) Decision {
	name, err := ParseName(raw)
	if err != nil {
		return quarantine("malformed")
	}
	if r == nil {
		return quarantine("unknown")
	}
	indexes := r.byAlias[name]
	switch len(indexes) {
	case 0:
		return quarantine("unknown")
	case 1:
		a := r.agreements[indexes[0]]
		return Decision{Decision: "forward", Route: &Route{
			Address: a.Border, Namespace: a.Namespace, AgreementID: a.ID, Rung: a.Rung,
		}}
	default:
		return quarantine("ambiguous")
	}
}

func quarantine(reason string) Decision {
	return Decision{Decision: "quarantine", Reason: reason}
}

func hasIndex(indexes []int, want int) bool {
	for _, got := range indexes {
		if got == want {
			return true
		}
	}
	return false
}
