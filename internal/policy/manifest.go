package policy

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

// ManifestVersion is the schema version of the emitted body. It is the lab's
// own version, not the prototype's 1.1.0-alpha: the key changed with the
// meaning (a declaration of what a surface sells and can enforce, not a
// persona's menu).
const ManifestVersion = "0.1.0"

// Enforcement values. A control the surface actually applies is "enforced";
// anything written down and relied on but not applied is "declared". The lab
// treats declared as legitimate as long as it is labelled — what it does not
// tolerate is a declaration read as a guarantee.
const (
	Enforced = "enforced"
	Declared = "declared"
)

// Manifest is the well-known document lab-safety publishes for a lab.
//
// The outer shape follows the barista prototype in issue #1 (issuer plus the
// endpoints an agent must walk to obtain a token); the extension key is the
// lab's own because the document is a policy declaration, not a persona.
type Manifest struct {
	Issuer                string `json:"issuer,omitempty"`
	AuthorizationEndpoint string `json:"authorization_endpoint,omitempty"`
	TokenEndpoint         string `json:"token_endpoint,omitempty"`
	RegistrationEndpoint  string `json:"registration_endpoint,omitempty"`

	Body Body `json:"x_lab_safety_manifest"`
}

// Body is the policy object itself.
type Body struct {
	Version string `json:"version"`
	// Status is "declared" while any part of the object depends on a gate the
	// lab does not run. It becomes "enforced" only when every gate is checked.
	Status  string   `json:"status"`
	Note    string   `json:"note,omitempty"`
	Sources []string `json:"generated_from,omitempty"`

	// Skills are the priced tear-sheet skills: dynamic or static tools a caller
	// may only reach with a token. The prototype called these the secret menu.
	Skills []Skill `json:"skills"`

	Surfaces []Surface `json:"surfaces"`

	Timing *Timing `json:"timing,omitempty"`
	Grants *Grants `json:"grants,omitempty"`
}

// Skill is one priced skill in the manifest.
type Skill struct {
	Scope   string   `json:"scope"`
	Tool    string   `json:"tool"`
	Surface string   `json:"surface,omitempty"`
	Classes []string `json:"classes,omitempty"`
	// MinRung is the highest declared minimum rung among the tool's classes.
	MinRung int `json:"min_rung"`
	// Dynamic marks a tool registered at runtime (adhd.cluster.join and the
	// like): the "custom skills that are dynamic" the price is meant to cover.
	Dynamic bool   `json:"dynamic,omitempty"`
	Price   Price  `json:"price"`
	Gate    Gate   `json:"gate"`
	Note    string `json:"note,omitempty"`
}

// Gate says where a token is obtained and whether anything checks it.
type Gate struct {
	Endpoint    string `json:"endpoint,omitempty"`
	Enforcement string `json:"enforcement"`
	Note        string `json:"note,omitempty"`
}

// Surface is the manifest's view of one sheet.
type Surface struct {
	ID        string `json:"id"`
	Sheet     string `json:"sheet"`
	Owner     string `json:"owner,omitempty"`
	Endpoint  string `json:"endpoint,omitempty"`
	Transport string `json:"transport,omitempty"`
	Protocol  string `json:"protocol,omitempty"`

	Classes        []string       `json:"classes"`
	Tools          []Tool         `json:"tools"`
	MinRungByClass map[string]int `json:"min_rung_by_class,omitempty"`
	RungStatus     string         `json:"rung_status,omitempty"`
	RungEnforced   bool           `json:"rung_enforced"`
	Controls       []Control      `json:"controls,omitempty"`
}

// Build assembles the manifest from the sheets (what exists) and the policy
// (what is for sale). A skill whose tool no sheet declares is left with an
// empty surface; the checker reports it rather than this function inventing
// one.
func Build(sheets []Sheet, p *Policy) Manifest {
	m := Manifest{
		Issuer:                p.Issuer,
		AuthorizationEndpoint: p.AuthorizationEndpoint,
		TokenEndpoint:         p.TokenEndpoint,
		RegistrationEndpoint:  p.RegistrationEndpoint,
	}
	m.Body = Body{
		Version:  ManifestVersion,
		Status:   Declared,
		Skills:   []Skill{},
		Surfaces: []Surface{},
		Timing:   p.Timing,
		Note: "Derived from the safety data sheets. A field marked \"declared\" is written down but not " +
			"applied by any surface; a price buys a credential, not a request.",
	}
	if p.Grants != (Grants{}) {
		m.Body.Grants = &p.Grants
	}

	for _, s := range sheets {
		m.Body.Sources = append(m.Body.Sources, s.File)
		m.Body.Surfaces = append(m.Body.Surfaces, surfaceFrom(s))
	}

	for _, decl := range p.Skills {
		enforcement := decl.Gate.Enforcement
		if enforcement != Enforced {
			enforcement = Declared
		}
		skill := Skill{
			Scope: decl.Scope,
			Tool:  decl.Tool,
			Price: decl.Price,
			Note:  decl.Note,
			Gate: Gate{
				Endpoint:    p.RegistrationEndpoint,
				Enforcement: enforcement,
				Note:        gateNote(enforcement, p.Grants.GrantTool),
			},
		}
		if sheet, tool, ok := findTool(sheets, decl.Tool); ok {
			skill.Surface = sheet.ID
			skill.Classes = tool.Classes
			skill.MinRung = maxRung(sheet.RungByClass, tool.Classes)
			skill.Dynamic = tool.Dynamic
		}
		m.Body.Skills = append(m.Body.Skills, skill)
	}

	// The object is only "enforced" if it prices something and every gate it
	// publishes is checked somewhere. Today no gate is, so this stays declared
	// and the manifest says so instead of implying a paywall that does not
	// exist.
	enforced := len(m.Body.Skills) > 0
	for _, s := range m.Body.Skills {
		if s.Gate.Enforcement != Enforced {
			enforced = false
		}
	}
	if enforced {
		m.Body.Status = Enforced
	}
	sort.Slice(m.Body.Skills, func(i, j int) bool { return m.Body.Skills[i].Scope < m.Body.Skills[j].Scope })
	return m
}

// gateNote states what backs the gate, in the manifest's own vocabulary: an
// enforced gate is one a live surface refuses, a declared one is a price with
// nothing behind it yet.
func gateNote(enforcement, grantTool string) string {
	if enforcement == Enforced {
		if grantTool == "" {
			return "claimed enforced, but no grant tool is declared to drive the check"
		}
		return "the surface refuses this tool without a token; `lab-safety check --probe --gate` verifies it"
	}
	if grantTool != "" {
		return "the token is issued by " + grantTool + " but no surface is known to read it"
	}
	return "the token this skill requires is not issued by any surface in the lab yet"
}

// WriteJSON encodes the manifest with indentation.
func (m Manifest) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(m); err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	return nil
}

func surfaceFrom(s Sheet) Surface {
	return Surface{
		ID:             s.ID,
		Sheet:          s.File,
		Owner:          s.Owner,
		Endpoint:       s.Endpoint,
		Transport:      s.Transport,
		Protocol:       s.Protocol,
		Classes:        s.Classes(),
		Tools:          s.Tools,
		MinRungByClass: s.RungByClass,
		RungStatus:     s.RungStatus,
		RungEnforced:   s.RungEnforced,
		Controls:       s.Controls,
	}
}

// findTool locates a declared tool by exact name across the sheets. Dynamic
// tools are matched by their declared pattern (e.g. adhd.project.<name>.call),
// because that is the name a sheet uses.
func findTool(sheets []Sheet, name string) (Sheet, Tool, bool) {
	for _, s := range sheets {
		for _, t := range s.Tools {
			if t.Name == name {
				return s, t, true
			}
		}
	}
	return Sheet{}, Tool{}, false
}

// maxRung returns the highest rung any of the tool's classes requires. A tool
// that is both R and E is gated as E.
func maxRung(byClass map[string]int, classes []string) int {
	highest := 0
	found := false
	for _, c := range classes {
		if r, ok := byClass[c]; ok {
			if !found || r > highest {
				highest = r
				found = true
			}
		}
	}
	return highest
}
