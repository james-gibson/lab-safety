package survey

import (
	"regexp"
	"strconv"
)

// Dataset is everything that differs between one survey and another: the
// projection, the vocabulary, the provenance rules, and the generated
// artifacts. The engine (schema, projection, drift, canonicalisation) is
// shared; a survey is this value + its root directory.
//
// The model survey (lab/survey) and the tool survey (lab/survey/tool) are the
// two datasets today. Adding a third is a value, not a fork.
type Dataset struct {
	Name    string // "model" | "tool"
	Root    string // default root relative to lab/survey
	IDField string // "agent_id" | "tool_id"
	IDRe    *regexp.Regexp

	FlatCount   int                               // properties in schemas/flat.json
	Mapping     []field                           // flat column -> nested path (the answer columns)
	Mapped      map[string]func(any) (any, error) // authored vocabulary classes
	Derive      func(map[string]string) map[string]string
	DerivedKeys []string

	SchemaVersion int
	SourceJSON    string // relative to Root
	SourceSHA     string // relative to Root

	CSVPath     string
	FeaturePath string
	ReadmePath  string
	ReadmeBegin string
	ReadmeEnd   string

	SummaryColumns []string
	FeatureHeader  string
}

// Model is the agent survey: lab/survey.
func Model() *Dataset {
	return &Dataset{
		Name:           "model",
		Root:           ".",
		IDField:        "agent_id",
		IDRe:           agentIDRe,
		FlatCount:      100,
		Mapping:        mapping,
		Derive:         modelDeriveCells,
		DerivedKeys:    DerivedKeys,
		SchemaVersion:  2,
		SourceJSON:     "sources/issue-3.json",
		SourceSHA:      "sources/issue-3.sha256",
		CSVPath:        CSVPath,
		FeaturePath:    FeaturePath,
		ReadmePath:     ReadmePath,
		ReadmeBegin:    readmeBegin,
		ReadmeEnd:      readmeEnd,
		SummaryColumns: SummaryColumns,
		FeatureHeader:  modelFeatureHeader,
	}
}

// Tool is the tool survey: lab/survey/tool.
func Tool() *Dataset {
	return &Dataset{
		Name:          "tool",
		Root:          "tool",
		IDField:       "tool_id",
		IDRe:          agentIDRe,
		FlatCount:     46,
		Mapping:       toolMapping,
		Mapped:        toolVocabulary(),
		Derive:        toolDeriveCells,
		DerivedKeys:   toolDerivedKeys,
		SchemaVersion: 1,
		SourceJSON:    "sources/issue-3.json",
		SourceSHA:     "sources/issue-3.sha256",
		CSVPath:       "tools.csv",
		FeaturePath:   "tools.feature",
		ReadmePath:    "README.md",
		ReadmeBegin:   "<!-- BEGIN GENERATED: tool-survey-summary -->",
		ReadmeEnd:     "<!-- END GENERATED: tool-survey-summary -->",
		SummaryColumns: []string{
			"tool_id", "tool_vendor", "primary_interface_type", "mutability_class",
			"blast_class", "complexity_class", "payload_class",
			"minimum_capability_rung", "max_acceptable_imaginary_distance",
			"supports_mcp", "is_idempotent", "human_in_the_loop_required",
			"risk_score", "review_by",
		},
		FeatureHeader: toolFeatureHeader,
	}
}

// mappingColumns returns the projected column names in projection order.
func mappingColumns(m []field) []string {
	out := make([]string, 0, len(m))
	for _, f := range m {
		out = append(out, f.Col)
	}
	return out
}

// modelDeriveCells is the model survey's four computed metrics as CSV cells.
func modelDeriveCells(c map[string]string) map[string]string {
	d := Derive(c)
	return map[string]string{
		"mcp_ready":        strconv.FormatBool(d.MCPReady),
		"autonomous_ready": strconv.FormatBool(d.AutonomousReady),
		"production_grade": strconv.FormatBool(d.ProductionGrade),
		"strictness_score": strconv.Itoa(d.StrictnessScore),
	}
}

const modelFeatureHeader = `@lab-safety
@model-survey
@z0-physical
@domain-model-survey
Feature: Model survey — agent integration profiles

  The survey observations are the origin; this file is their projection.
  profiles/<agent_id>.json is the source of truth: models.csv, the README
  summary, and every line below are generated from it, and ` + "`survey --check`" + `
  fails on any byte of drift.

  Which way the arrow points: a *surface* sheet (sheets/) declares what an MCP
  surface exposes, and the Gherkin there is the origin — a surface that does not
  satisfy it is a failing test. A *survey* spec is the opposite arrow: what an
  agent answered is the origin and this Gherkin is the projection of it.
  Nothing here is executed; a step declares, and ` + "`survey --check`" + ` decides what
  the declaration is worth.

  Background:
    Given the profiles under "profiles/" are the survey's source of truth

`

const toolFeatureHeader = `@lab-safety
@tool-survey
@z0-physical
@domain-tool-survey
Feature: Tool survey — tool integration profiles

  The survey observations are the origin; this file is their projection.
  profiles/<tool_id>.json is the source of truth: tools.csv, the README
  summary, and every line below are generated from it, and ` + "`survey --check`" + `
  fails on any byte of drift.

  Which way the arrow points: a *surface* sheet (sheets/) declares what an MCP
  surface exposes, and the Gherkin there is the origin — a surface that does not
  satisfy it is a failing test. A *survey* spec is the opposite arrow: what a
  tool answered is the origin and this Gherkin is the projection of it.
  Nothing here is executed; a step declares, and ` + "`survey --check`" + ` decides what
  the declaration is worth.

  Background:
    Given the profiles under "profiles/" are the survey's source of truth

`
