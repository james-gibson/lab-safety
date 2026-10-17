package survey

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Provenance is the authored evidence that a profile was observed rather than
// invented: where it came from, when it was checked, and when it must be
// re-checked. It is the only part of a survey a human writes on purpose;
// everything else is an observation or a projection.
type Provenance struct {
	SourceURL     string `json:"source_url"`
	CheckedAt     string `json:"checked_at"`
	ReviewBy      string `json:"review_by"`
	SurveyMethod  string `json:"survey_method"`
	SchemaVersion int    `json:"schema_version"`
}

// Profile is one surveyed agent. Answer holds the nested reply (normalised at
// import); Provenance carries the evidence.
type Profile struct {
	AgentID    string
	Path       string
	Provenance Provenance
	Answer     map[string]any
}

type profileDoc struct {
	Provenance Provenance     `json:"provenance"`
	Answer     map[string]any `json:"answer"`
}

// agentIDRe is the id rule: the file name is the identifier, lowercase
// kebab-case (dots allowed for versions, as in gemini-1.5-pro), so an id is
// addressable and cannot drift from its file.
var agentIDRe = regexp.MustCompile(`^[a-z0-9]+([-.][a-z0-9]+)*$`)

// DerivedKeys are computed by the tool and must never be authored.
var DerivedKeys = []string{"mcp_ready", "autonomous_ready", "production_grade", "strictness_score"}

// LoadProfiles reads the model survey's profiles/*.json under root.
func LoadProfiles(root string) ([]Profile, []string, error) {
	return LoadProfilesWith(Model(), root)
}

// LoadProfilesWith reads a dataset's profiles/*.json under root, decoding each
// strictly, and returns them sorted by ID together with per-file problems (a
// bad file is reported, not fatal — Check reports every failure it can).
func LoadProfilesWith(ds *Dataset, root string) ([]Profile, []string, error) {
	dir := filepath.Join(root, "profiles")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	var names []string
	var problems []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	profiles := make([]Profile, 0, len(names))
	seen := map[string]string{}
	for _, name := range names {
		if !strings.HasSuffix(name, ".json") {
			problems = append(problems, fmt.Sprintf("orphan profile: profiles/%s is not a .json profile", name))
			continue
		}
		id := strings.TrimSuffix(name, ".json")
		if !ds.IDRe.MatchString(id) {
			problems = append(problems, fmt.Sprintf("orphan profile: %q is not a kebab-case %s", id, ds.IDField))
			continue
		}
		if prev, ok := seen[id]; ok {
			problems = append(problems, fmt.Sprintf("duplicate %s %q: %s and %s", ds.IDField, id, prev, name))
			continue
		}
		seen[id] = name

		path := filepath.Join(dir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, problems, err
		}
		p, err := decodeProfile(id, path, raw)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%v", err))
			continue
		}
		profiles = append(profiles, p)
	}
	return profiles, problems, nil
}

// decodeProfile strictly decodes one profile document: unknown keys at the
// envelope and in the provenance are errors. The answer is policed by
// nested.json, which declares additionalProperties false throughout.
func decodeProfile(id, path string, raw []byte) (Profile, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var doc profileDoc
	if err := dec.Decode(&doc); err != nil {
		return Profile{}, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	if dec.More() {
		return Profile{}, fmt.Errorf("%s: trailing content after the profile", filepath.Base(path))
	}
	if doc.Answer == nil {
		return Profile{}, fmt.Errorf("%s: no \"answer\" object", filepath.Base(path))
	}
	return Profile{AgentID: id, Path: path, Provenance: doc.Provenance, Answer: doc.Answer}, nil
}

// CheckProvenance validates the model survey's authored evidence.
func (p Profile) CheckProvenance(knownSources map[string]bool) []string {
	return p.CheckProvenanceWith(Model(), knownSources)
}

// CheckProvenanceWith validates the authored evidence for a dataset: the fields
// a JSON Schema does not express — date ordering, the re-check window, the
// source's existence, and the ban on authoring a derived metric.
func (p Profile) CheckProvenanceWith(ds *Dataset, knownSources map[string]bool) []string {
	var out []string
	pr := p.Provenance
	for _, f := range []struct{ name, val string }{
		{"source_url", pr.SourceURL},
		{"checked_at", pr.CheckedAt},
		{"review_by", pr.ReviewBy},
		{"survey_method", pr.SurveyMethod},
	} {
		if f.val == "" {
			out = append(out, fmt.Sprintf("provenance: %s has no %s", p.AgentID, f.name))
		}
	}
	checked, checkedErr := time.Parse("2006-01-02", pr.CheckedAt)
	if checkedErr != nil && pr.CheckedAt != "" {
		out = append(out, fmt.Sprintf("provenance: %s checked_at %q is not an ISO date", p.AgentID, pr.CheckedAt))
	}
	review, reviewErr := time.Parse("2006-01-02", pr.ReviewBy)
	if reviewErr != nil && pr.ReviewBy != "" {
		out = append(out, fmt.Sprintf("provenance: %s review_by %q is not an ISO date", p.AgentID, pr.ReviewBy))
	}
	if checkedErr == nil && reviewErr == nil && !review.After(checked) {
		out = append(out, fmt.Sprintf("provenance: %s review_by %s is not after checked_at %s",
			p.AgentID, pr.ReviewBy, pr.CheckedAt))
	}
	switch pr.SurveyMethod {
	case "self-report", "documentation", "observed", "":
	default:
		out = append(out, fmt.Sprintf("provenance: %s survey_method %q is not one of self-report|documentation|observed",
			p.AgentID, pr.SurveyMethod))
	}
	if pr.SchemaVersion != ds.SchemaVersion {
		out = append(out, fmt.Sprintf("provenance: %s schema_version is %d, want %d",
			p.AgentID, pr.SchemaVersion, ds.SchemaVersion))
	}
	if pr.SourceURL != "" && knownSources != nil && !knownSources[pr.SourceURL] {
		out = append(out, fmt.Sprintf("provenance: %s source_url %s is not in %s",
			p.AgentID, pr.SourceURL, ds.SourceJSON))
	}
	for _, k := range ds.DerivedKeys {
		if _, ok := p.Answer[k]; ok {
			out = append(out, fmt.Sprintf("derived: %s authors %s; the tool computes it", p.AgentID, k))
		}
	}
	return out
}

// SourceURLs reads the model survey's sources/issue-3.json and returns the set
// of comment permalinks it carries.
func SourceURLs(root string) (map[string]bool, error) {
	return SourceURLsAt(root, "sources/issue-3.json")
}

// SourceURLsAt reads a source export at root/rel and returns the set of comment
// permalinks it carries, so a profile's source_url can be checked against the
// raw export rather than taken on trust.
func SourceURLsAt(root, rel string) (map[string]bool, error) {
	raw, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return nil, err
	}
	var doc struct {
		Comments []struct {
			URL string `json:"url"`
		} `json:"comments"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, c := range doc.Comments {
		out[c.URL] = true
	}
	return out, nil
}

// SourceDigest verifies the model survey's source export against its sidecar.
func SourceDigest(root string) error {
	return SourceDigestAt(root, "sources/issue-3.json", "sources/issue-3.sha256")
}

// SourceDigestAt verifies a source export against its sidecar. The export is the
// raw material the survey was normalised from; if it can be edited silently, no
// check downstream means anything.
func SourceDigestAt(root, jsonRel, shaRel string) error {
	body, err := os.ReadFile(filepath.Join(root, jsonRel))
	if err != nil {
		return err
	}
	sidecar, err := os.ReadFile(filepath.Join(root, shaRel))
	if err != nil {
		return err
	}
	fields := strings.Fields(string(sidecar))
	if len(fields) == 0 {
		return fmt.Errorf("%s is empty", shaRel)
	}
	got := sha256.Sum256(body)
	if fields[0] != hex.EncodeToString(got[:]) {
		return fmt.Errorf("%s does not match %s (the raw export was edited)", jsonRel, shaRel)
	}
	return nil
}
