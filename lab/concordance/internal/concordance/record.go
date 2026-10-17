package concordance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Source is a third party whose claims we compare against. It records where the
// claims live, the terms they live under, and — when the terms require it — the
// attribution that must travel with every view of the data.
type Source struct {
	ID          string  `json:"id"`
	URL         string  `json:"url"`
	TermsURL    string  `json:"terms_url"`
	Licence     string  `json:"licence"`
	Policy      string  `json:"policy"`
	RetrievedAt string  `json:"retrieved_at"`
	Credit      *Credit `json:"credit"`
	LinkPerRow  bool    `json:"listing_link_required"`
	Waiver      string  `json:"waiver"`
	Notes       string  `json:"notes"`
}

// Credit is the attribution a source's licence requires. If a source declares
// one, Check requires it in every artifact that shows the source's data.
type Credit struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

// licences are the terms we are willing to name. "unknown" is honest; a licence
// we cannot name is not a licence we may lean on.
var licences = []string{"public-domain", "permissive", "attribution-required", "all-rights-reserved", "unknown"}

// Record is one comparison: a subject both sides name, a predicate, and the two
// claims. Agreement is NOT a field — it is computed by Project.
type Record struct {
	Path       string // module-relative; the path IS the key
	SourceID   string
	SubjectID  string
	Predicate  string
	Source     SourceRef
	Subject    SubjectRef
	TheirClaim Claim
	OurClaim   OwnClaim
	Provenance Provenance
}

type SourceRef struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	RetrievedAt string `json:"retrieved_at"`
}

type SubjectRef struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	ListingURL string `json:"listing_url"`
}

type Claim struct {
	Value string `json:"value"`
}

type OwnClaim struct {
	Value string `json:"value"`
	Basis string `json:"basis"`
}

type Provenance struct {
	CheckedAt string `json:"checked_at"`
	ReviewBy  string `json:"review_by"`
	Method    string `json:"method"`
	Notes     string `json:"notes"`
}

type recordDoc struct {
	Source     SourceRef  `json:"source"`
	Subject    SubjectRef `json:"subject"`
	Predicate  string     `json:"predicate"`
	TheirClaim Claim      `json:"their_claim"`
	OurClaim   OwnClaim   `json:"our_claim"`
	Provenance Provenance `json:"provenance"`
}

var (
	idRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
)

// loadSources reads sources/*.json. A bad declaration is reported, not fatal:
// Check should report everything it can see in one pass.
func (d *Dataset) loadSources() error {
	dir := filepath.Join(d.Root, "sources")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	d.Sources = map[string]Source{}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		var s Source
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&s); err != nil {
			d.problems = append(d.problems, fmt.Sprintf("sources/%s: %v", name, err))
			continue
		}
		id := strings.TrimSuffix(name, ".json")
		switch {
		case !idRe.MatchString(s.ID):
			d.problems = append(d.problems, fmt.Sprintf("sources/%s: id %q is not kebab-case", name, s.ID))
		case s.ID != id:
			d.problems = append(d.problems, fmt.Sprintf("sources/%s declares id %q — the file name is the id", name, s.ID))
		case !contains(licences, s.Licence):
			d.problems = append(d.problems, fmt.Sprintf("sources/%s: licence %q is not one of %v", name, s.Licence, licences))
		case s.Policy != "pointer":
			d.problems = append(d.problems, fmt.Sprintf("sources/%s: policy %q is not \"pointer\" — anything else would let their content in", name, s.Policy))
		}
		d.Sources[s.ID] = s
	}
	return nil
}

// loadRecords walks records/<source>/<subject>/<predicate>.json. The path is
// the key: the three levels must equal the three fields, so a record cannot be
// addressed two ways.
func (d *Dataset) loadRecords() error {
	root := filepath.Join(d.Root, "records")
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	var paths []string
	for _, src := range entries {
		if !src.IsDir() {
			continue
		}
		subs, err := os.ReadDir(filepath.Join(root, src.Name()))
		if err != nil {
			return err
		}
		for _, sub := range subs {
			if !sub.IsDir() {
				continue
			}
			preds, err := os.ReadDir(filepath.Join(root, src.Name(), sub.Name()))
			if err != nil {
				return err
			}
			for _, p := range preds {
				if p.IsDir() || !strings.HasSuffix(p.Name(), ".json") {
					d.problems = append(d.problems,
						fmt.Sprintf("orphan record: records/%s/%s/%s is not a .json record", src.Name(), sub.Name(), p.Name()))
					continue
				}
				paths = append(paths, filepath.Join(src.Name(), sub.Name(), strings.TrimSuffix(p.Name(), ".json")))
			}
		}
	}
	sort.Strings(paths)
	seen := map[string]bool{}
	for _, key := range paths {
		if seen[key] {
			d.problems = append(d.problems, fmt.Sprintf("duplicate record: %s appears twice", key))
			continue
		}
		seen[key] = true
		rel := filepath.Join("records", key+".json")
		raw, err := os.ReadFile(filepath.Join(d.Root, rel))
		if err != nil {
			return err
		}
		rec, err := d.decodeRecord(rel, raw)
		if err != nil {
			d.problems = append(d.problems, err.Error())
			continue
		}
		d.Records = append(d.Records, rec)
	}
	return nil
}

// decodeRecord decodes one record strictly: an unknown key is an error, which
// is how authoring `agrees` is refused rather than quietly ignored.
func (d *Dataset) decodeRecord(rel string, raw []byte) (Record, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var doc recordDoc
	if err := dec.Decode(&doc); err != nil {
		return Record{}, fmt.Errorf("%s: %w", rel, err)
	}
	inst, err := roundTrip(raw)
	if err != nil {
		return Record{}, fmt.Errorf("%s: %w", rel, err)
	}
	if err := d.Record.Validate(inst); err != nil {
		return Record{}, fmt.Errorf("%s: %s", rel, oneLine(err))
	}
	return Record{
		Path:       rel,
		SourceID:   doc.Source.ID,
		SubjectID:  doc.Subject.ID,
		Predicate:  doc.Predicate,
		Source:     doc.Source,
		Subject:    doc.Subject,
		TheirClaim: doc.TheirClaim,
		OurClaim:   doc.OurClaim,
		Provenance: doc.Provenance,
	}, nil
}

// Key is the record's address: source / subject / predicate.
func (r Record) Key() string {
	return r.SourceID + "/" + r.SubjectID + "/" + r.Predicate
}

// roundTrip puts a Go value through JSON the way the validator expects it.
func roundTrip(v any) (any, error) {
	var raw []byte
	switch t := v.(type) {
	case []byte:
		raw = t
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		raw = b
	}
	return unmarshal(raw)
}

func oneLine(err error) string {
	return strings.Join(strings.Split(err.Error(), "\n"), " | ")
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// CheckProvenance validates the parts a schema cannot express.
func (r Record) CheckProvenance() []string {
	var out []string
	p := r.Provenance
	checked, checkedErr := time.Parse("2006-01-02", p.CheckedAt)
	if checkedErr != nil {
		out = append(out, fmt.Sprintf("provenance: %s checked_at %q is not an ISO date", r.Key(), p.CheckedAt))
	}
	review, reviewErr := time.Parse("2006-01-02", p.ReviewBy)
	if reviewErr != nil {
		out = append(out, fmt.Sprintf("provenance: %s review_by %q is not an ISO date", r.Key(), p.ReviewBy))
	}
	retrieved, retrievedErr := time.Parse("2006-01-02", r.Source.RetrievedAt)
	if retrievedErr != nil {
		out = append(out, fmt.Sprintf("provenance: %s retrieved_at %q is not an ISO date", r.Key(), r.Source.RetrievedAt))
	}
	if checkedErr == nil && reviewErr == nil && !review.After(checked) {
		out = append(out, fmt.Sprintf("provenance: %s review_by %s is not after checked_at %s", r.Key(), p.ReviewBy, p.CheckedAt))
	}
	if checkedErr == nil && retrievedErr == nil && retrieved.After(checked) {
		out = append(out, fmt.Sprintf("provenance: %s retrieved_at %s is after checked_at %s — the claim was read after we wrote it down",
			r.Key(), r.Source.RetrievedAt, p.CheckedAt))
	}
	if p.Method != "observed" && p.Method != "read" {
		out = append(out, fmt.Sprintf("provenance: %s method %q is not one of observed|read", r.Key(), p.Method))
	}
	return out
}
