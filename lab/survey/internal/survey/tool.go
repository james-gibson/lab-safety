package survey

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// The tool survey's projection. The answer is preserved verbatim; the flat row
// adds the authored vocabulary classes (the columns in toolVocabulary) and the
// computed metrics. The raw freeform answer is never rewritten, so a class can
// be argued with by reading the answer beside it.
var toolMapping = []field{
	{"tool_name", "toolName", kScalar},
	{"tool_vendor", "toolVendorOrProject", kScalar},
	{"tool_version", "toolVersion", kScalar},
	{"description", "description", kScalar},
	{"minimum_capability_rung", "labSafetyFitnessRequirements.minimumCapabilityRung", kScalar},
	{"max_acceptable_imaginary_distance", "labSafetyFitnessRequirements.maxAcceptableImaginaryDistance", kScalar},
	{"acceptable_direction_labels", "labSafetyFitnessRequirements.acceptableDirectionLabels", kList},
	{"requires_seed42_ground_truth", "labSafetyFitnessRequirements.requiresSeed42GroundTruthAdherence", kScalar},
	{"consequential_operations", "labSafetyFitnessRequirements.consequentialOperationsExposed", kList},
	{"fundamental_failures", "labSafetyFitnessRequirements.fundamentalFailuresThatDisqualify", kList},

	{"is_idempotent", "toolExecutionProfile.isIdempotent", kScalar},
	{"supports_dry_run", "toolExecutionProfile.supportsDryRunMode", kScalar},
	{"state_mutability", "toolExecutionProfile.stateMutability", kScalar},
	{"mutability_class", "toolExecutionProfile.stateMutability", kMappedClass},
	{"blast_radius", "toolExecutionProfile.blastRadius", kScalar},
	{"blast_class", "toolExecutionProfile.blastRadius", kMappedClass},

	{"primary_interface_type", "interfaceAndSchema.primaryInterfaceType", kScalar},
	{"interface_families", "interfaceAndSchema.primaryInterfaceType", kMappedList},
	{"schema_standard", "interfaceAndSchema.schemaStandard", kScalar},
	{"schema_families", "interfaceAndSchema.schemaStandard", kMappedList},
	{"parameter_complexity", "interfaceAndSchema.parameterComplexity", kScalar},
	{"complexity_class", "interfaceAndSchema.parameterComplexity", kMappedClass},
	{"payload_size_sensitivity", "interfaceAndSchema.payloadSizeSensitivity", kScalar},
	{"payload_class", "interfaceAndSchema.payloadSizeSensitivity", kMappedClass},

	{"auth_mechanism", "authAndVerification.authMechanism", kScalar},
	{"auth_families", "authAndVerification.authMechanism", kMappedList},
	{"requires_independent_alarm_verification", "authAndVerification.requiresIndependentAlarmVerification", kScalar},
	{"human_in_the_loop_required", "authAndVerification.humanInTheLoopRequiredForConsequentialSteps", kScalar},

	{"primary_use_cases", "agentIntegrationDetails.primaryUseCases", kList},
	{"common_agent_failure_modes", "agentIntegrationDetails.commonAgentFailureModes", kList},
	{"recovery_mechanisms", "agentIntegrationDetails.recoveryMechanisms", kList},

	{"supports_mcp", "compatibility.supportsMCP", kScalar},
	{"requires_strict_typing", "compatibility.requiresStrictTyping", kScalar},
	{"supported_model_families", "compatibility.supportedModelFamilies", kList},
	{"model_families", "compatibility.supportedModelFamilies", kMappedList},

	{"notes", "notes", kScalar},
}

var toolDerivedKeys = []string{"mcp_ready", "dry_run_ready", "autonomous_ready", "risk_score"}

// toolVocabulary maps each derived class column to the function that turns the
// raw answer into the authored vocabulary. The map is total over the survey's
// current answers; an answer no mapping covers fails --check, which is the
// point: a new freeform value must be classified, not silently passed through.
func toolVocabulary() map[string]func(any) (any, error) {
	return map[string]func(any) (any, error){
		"mutability_class":   classFromFirstWord([]string{"none", "low", "moderate", "high", "critical", "variable"}),
		"complexity_class":   classFromFirstWord([]string{"low", "moderate", "high", "extreme"}),
		"payload_class":      classFromFirstWord([]string{"low", "moderate", "high"}),
		"blast_class":        tableClass(blastTable),
		"interface_families": tableList(interfaceTable),
		"schema_families":    tableList(schemaTable),
		"auth_families":      tableList(authTable),
		"model_families":     mapModelFamilies,
	}
}

func asStr(v any) (string, error) {
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("want a string, got %T", v)
	}
	return strings.TrimSpace(s), nil
}

func firstWord(s string) string {
	fields := strings.Fields(strings.ToLower(s))
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(fields[0], "(),;:")
}

// classFromFirstWord normalises a freeform "High (because …)" answer to its
// leading class. It is deliberately narrow: "High" is a class, "Extreme" is not
// unless the survey's vocabulary says so.
func classFromFirstWord(vocab []string) func(any) (any, error) {
	allowed := map[string]bool{}
	for _, v := range vocab {
		allowed[v] = true
	}
	return func(v any) (any, error) {
		s, err := asStr(v)
		if err != nil {
			return nil, err
		}
		w := firstWord(s)
		if !allowed[w] {
			return nil, fmt.Errorf("no class mapping for %q (leading word %q; want one of %v)", s, w, vocab)
		}
		return w, nil
	}
}

func tableClass(table map[string]string) func(any) (any, error) {
	return func(v any) (any, error) {
		s, err := asStr(v)
		if err != nil {
			return nil, err
		}
		if c, ok := table[strings.ToLower(s)]; ok {
			return c, nil
		}
		return nil, fmt.Errorf("no class mapping for %q", s)
	}
}

func tableList(table map[string][]string) func(any) (any, error) {
	return func(v any) (any, error) {
		s, err := asStr(v)
		if err != nil {
			return nil, err
		}
		if c, ok := table[strings.ToLower(s)]; ok {
			return joinSet(c), nil
		}
		return nil, fmt.Errorf("no list mapping for %q", s)
	}
}

func mapModelFamilies(v any) (any, error) {
	items, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("want a list, got %T", v)
	}
	var out []string
	for _, it := range items {
		s, err := asStr(it)
		if err != nil {
			return nil, err
		}
		out = append(out, modelFamily(s))
	}
	return joinSet(out), nil
}

func modelFamily(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch {
	case s == "all":
		return "all"
	case strings.Contains(s, "codex"):
		return "codex"
	case strings.Contains(s, "claude"):
		return "claude"
	case strings.Contains(s, "gemini"):
		return "gemini"
	case strings.Contains(s, "gpt"), strings.Contains(s, "openai"):
		return "gpt"
	default:
		return "other"
	}
}

func joinSet(items []string) string {
	seen := map[string]bool{}
	out := make([]string, 0, len(items))
	for _, it := range items {
		it = strings.TrimSpace(it)
		if it == "" || seen[it] {
			continue
		}
		seen[it] = true
		out = append(out, it)
	}
	sort.Strings(out)
	return strings.Join(out, "|")
}

var blastTable = map[string]string{
	"none":                                         "none",
	"user workspace / local machine":               "local",
	"project psi / local ide state":                "local",
	"project / team workspace":                     "project",
	"repository / organization / infrastructure":   "org",
	"user cloud storage / organizational data":     "org",
	"monorepo / ci-cd pipeline":                    "org",
	"enterprise communication / workspace culture": "org",
	"application data / graph state":               "data",
	"database instance / application data":         "data",
	"user profile / music catalog":                 "data",
	"physical logistics / commercial liability":    "physical",
	"production site / global cdn":                 "production",
}

var interfaceTable = map[string][]string{
	"vs code extension api (ts/js)":     {"sdk"},
	"rest / graphql":                    {"rest", "graphql"},
	"rest":                              {"rest"},
	"java/kotlin api":                   {"sdk"},
	"mcp / graphql":                     {"mcp", "graphql"},
	"cli / mcp server":                  {"cli", "mcp"},
	"wire protocol (sql) / mcp tooling": {"sql", "mcp"},
	"web api / events api / mcp":        {"rest", "events", "mcp"},
	"rest api / cli":                    {"rest", "cli"},
}

var schemaTable = map[string][]string{
	"typescript definition files (.d.ts)":       {"typescript"},
	"openapi 3.0 / graphql schema":              {"openapi", "graphql"},
	"openapi / adf (atlassian document format)": {"openapi", "custom"},
	"plugin xml / extension points":             {"xml"},
	"graphql sdl / introspection json":          {"graphql", "custom"},
	"custom json":                               {"custom"},
	"openapi / discovery service":               {"openapi"},
	"custom json (strict validation)":           {"custom"},
	"json schema (project.json / nx.json)":      {"json-schema"},
	"ansi sql / jsonb schema":                   {"sql", "json-schema"},
	"block kit (json) / slack api schema":       {"custom"},
	"openapi / custom json":                     {"openapi", "custom"},
}

var authTable = map[string][]string{
	"oauth2 / extension identity":                    {"oauth2"},
	"personal access token (pat) / github app (jwt)": {"pat", "jwt"},
	"oauth 2.0 (3lo) / basic auth (api token)":       {"oauth2", "api-key"},
	"jetbrains account / hub":                        {"account"},
	"api key / oauth2 / persisted query safelisting": {"api-key", "oauth2", "custom"},
	"api key":                                {"api-key"},
	"oauth 2.0 (scopes-based)":               {"oauth2"},
	"jwt / oauth2":                           {"jwt", "oauth2"},
	"nx cloud token / local fs auth":         {"pat", "account"},
	"iam roles / connection strings / scram": {"iam"},
	"oauth 2.1 (pkce required)":              {"oauth2"},
	"oauth 2.0 (scopes) / bot token":         {"oauth2", "pat"},
	"personal access token (pat) / oauth2":   {"pat", "oauth2"},
}

// toolDeriveCells recomputes the four metrics from the row's cells, exactly the
// way --check recomputes them from tools.csv.
func toolDeriveCells(c map[string]string) map[string]string {
	risk := 0
	risk += weight(c["mutability_class"], map[string]int{
		"none": 0, "low": 1, "moderate": 2, "variable": 2, "high": 3, "critical": 4,
	})
	risk += weight(c["blast_class"], map[string]int{
		"none": 0, "local": 1, "project": 2, "org": 3, "data": 3, "production": 4, "physical": 5,
	})
	risk += weight(c["complexity_class"], map[string]int{
		"low": 1, "moderate": 2, "high": 3, "extreme": 4,
	})
	return map[string]string{
		"mcp_ready":        strconv.FormatBool(c["supports_mcp"] == "true"),
		"dry_run_ready":    strconv.FormatBool(c["supports_dry_run"] == "true"),
		"autonomous_ready": strconv.FormatBool(c["human_in_the_loop_required"] == "false" && c["is_idempotent"] == "true"),
		"risk_score":       strconv.Itoa(risk),
	}
}

func weight(class string, table map[string]int) int {
	if n, ok := table[class]; ok {
		return n
	}
	return 0
}
