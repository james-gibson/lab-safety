package survey

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// kind is how a flat column is encoded from the nested answer.
type kind int

const (
	kScalar  kind = iota // one string, boolean, integer or null
	kList                // a list of strings, pipe-encoded into one cell
	kIntList             // a list of integers, pipe-encoded into one cell
)

// field is one flat column and the nested path it projects from.
type field struct {
	Col  string
	Path string
	Kind kind
}

// mapping is the projection: flat column -> nested path. It covers every flat
// column except agent_id (the file name) and the nine fields the tool adds
// (five authored provenance, four derived). Check asserts that
// ["agent_id"] + mapping == flat.Order[:91], so the issue's 91 columns are
// exactly the projection of the answer and cannot silently grow or reorder.
var mapping = []field{
	{"agent_vendor", "agentVendorOrProject", kScalar},
	{"agent_version", "agentVersion", kScalar},
	{"runtime", "runtime", kScalar},
	{"model_family", "modelFamily", kScalar},
	{"deployment_type", "deploymentType", kScalar},
	{"can_inspect_private_endpoints", "canInspectPrivateEndpoints", kScalar},
	{"preferred_interface", "endpointPreferences.preferredInterfaceType", kScalar},
	{"supported_interfaces", "endpointPreferences.supportedInterfaceTypes", kList},
	{"preferred_transport", "endpointPreferences.preferredTransport", kScalar},
	{"supports_streaming", "endpointPreferences.supportsStreaming", kScalar},
	{"versioning_style", "endpointPreferences.preferredVersioningStyle", kScalar},
	{"auth_preferred", "auth.preferredMethod", kScalar},
	{"auth_acceptable", "auth.acceptableMethods", kList},
	{"auth_requires_human", "auth.requiresHumanInTheLoopForAuth", kScalar},
	{"request_format", "schemaAndContracts.preferredRequestFormat", kScalar},
	{"response_format", "schemaAndContracts.preferredResponseFormat", kScalar},
	{"strict_typing", "schemaAndContracts.requiresStrictTyping", kScalar},
	{"machine_readable_spec", "schemaAndContracts.requiresMachineReadableSpec", kScalar},
	{"preferred_specs", "schemaAndContracts.preferredSpecFormats", kList},
	{"needs_examples", "schemaAndContracts.needsExamples", kScalar},
	{"stable_field_names", "schemaAndContracts.needsStableFieldNames", kScalar},
	{"stable_enums", "schemaAndContracts.needsStableEnumValues", kScalar},
	{"read_only_pref", "operations.readOnlyPreferred", kScalar},
	{"write_capable_pref", "operations.writeCapablePreferred", kScalar},
	{"needs_discovery", "operations.needsDiscovery", kScalar},
	{"needs_pagination", "operations.needsPagination", kScalar},
	{"needs_filtering", "operations.needsFiltering", kScalar},
	{"needs_search", "operations.needsSearch", kScalar},
	{"needs_sorting", "operations.needsSorting", kScalar},
	{"needs_batch", "operations.needsBatchOperations", kScalar},
	{"needs_idempotent_writes", "operations.needsIdempotentWrites", kScalar},
	{"needs_file_upload", "operations.needsFileUpload", kScalar},
	{"needs_file_download", "operations.needsFileDownload", kScalar},
	{"needs_async_jobs", "operations.needsAsyncJobs", kScalar},
	{"error_format", "errorsAndRecovery.preferredErrorFormat", kScalar},
	{"standard_http_codes", "errorsAndRecovery.expectsStandardHttpStatusCodes", kScalar},
	{"structured_errors", "errorsAndRecovery.needsStructuredErrors", kScalar},
	{"validation_error_details", "errorsAndRecovery.needsValidationErrorDetails", kScalar},
	{"retryable_signals", "errorsAndRecovery.needsRetryableErrorSignals", kScalar},
	{"retryable_codes", "errorsAndRecovery.retryableStatusCodes", kIntList},
	{"needs_retries", "resilience.needsRetries", kScalar},
	{"needs_retry_after", "resilience.needsRetryAfterHandling", kScalar},
	{"needs_rate_limit_handling", "resilience.needsRateLimitHandling", kScalar},
	{"backoff_strategy", "resilience.preferredBackoffStrategy", kScalar},
	{"needs_timeout_guidance", "resilience.needsTimeoutGuidance", kScalar},
	{"needs_circuit_breaker", "resilience.needsCircuitBreakerBehavior", kScalar},
	{"latency_pref", "performance.latencyPreference", kScalar},
	{"throughput_pref", "performance.throughputPreference", kScalar},
	{"payload_sensitivity", "performance.payloadSizeSensitivity", kScalar},
	{"concurrency_sensitivity", "performance.concurrencySensitivity", kScalar},
	{"rate_limit_pref", "performance.rateLimitPreference", kScalar},
	{"needs_conversation_history", "contextAndMemory.needsConversationHistory", kScalar},
	{"needs_persistent_memory", "contextAndMemory.needsPersistentMemory", kScalar},
	{"needs_stateful_sessions", "contextAndMemory.needsStatefulSessions", kScalar},
	{"needs_external_context", "contextAndMemory.needsExternalContextInjection", kScalar},
	{"context_window_behavior", "contextAndMemory.preferredContextWindowBehavior", kScalar},
	{"context_loss_failure_mode", "contextAndMemory.contextLossFailureMode", kScalar},
	{"needs_request_ids", "observability.needsRequestIds", kScalar},
	{"needs_trace_ids", "observability.needsTraceIds", kScalar},
	{"needs_logs", "observability.needsLogs", kScalar},
	{"needs_audit_trail", "observability.needsAuditTrail", kScalar},
	{"needs_webhooks", "observability.needsCallbacksOrWebhooks", kScalar},
	{"needs_openapi", "documentationNeeds.needsOpenAPI", kScalar},
	{"needs_mcp", "documentationNeeds.needsMCP", kScalar},
	{"needs_a2a", "documentationNeeds.needsA2A", kScalar},
	{"needs_acp", "documentationNeeds.needsACP", kScalar},
	{"needs_human_docs", "documentationNeeds.needsHumanReadableDocs", kScalar},
	{"needs_sdk", "documentationNeeds.needsSDK", kScalar},
	{"needs_postman", "documentationNeeds.needsPostmanCollection", kScalar},
	{"backward_compatible", "compatibility.backwardCompatiblePreferred", kScalar},
	{"deprecation_policy", "compatibility.deprecationPolicyPreference", kScalar},
	{"breaking_change_tolerance", "compatibility.breakingChangeTolerance", kScalar},
	{"version_pinning", "compatibility.versionPinningPreference", kScalar},
	{"sensitive_data_handling", "securityAndPrivacy.sensitiveDataHandling", kScalar},
	{"pii_concerns", "securityAndPrivacy.piiConcerns", kList},
	{"secret_handling", "securityAndPrivacy.secretHandlingRules", kList},
	{"sandboxing_needs", "securityAndPrivacy.sandboxingNeeds", kScalar},
	{"data_retention_concerns", "securityAndPrivacy.dataRetentionConcerns", kScalar},
	{"changes_by_model", "deploymentSensitivity.changesByModelFamily", kScalar},
	{"changes_by_deployment", "deploymentSensitivity.changesByDeploymentType", kScalar},
	{"changes_by_org_size", "deploymentSensitivity.changesByOrganizationSize", kScalar},
	{"changes_by_use_case", "deploymentSensitivity.changesByUseCase", kScalar},
	{"must_have", "idealEndpointChecklist.mustHave", kList},
	{"nice_to_have", "idealEndpointChecklist.niceToHave", kList},
	{"deal_breakers", "idealEndpointChecklist.dealBreakers", kList},
	{"access_can_inspect", "accessLimitations.whatYouCanInspect", kList},
	{"access_cannot_inspect", "accessLimitations.whatYouCannotInspect", kList},
	{"assumptions", "accessLimitations.assumptionsYouAreMaking", kList},
	{"free_form_notes", "freeFormNotes", kScalar},
	{"caveats", "caveats", kList},
}

// Row is one projected flat row. Values are the typed JSON values the flat
// schema validates (bool, int, string, nil); CSVOf renders the CSV cells.
type Row struct {
	AgentID string
	Values  map[string]any
}

// Project turns a profile into a flat row and computes the derived metrics.
// It does not consult the profile for the derived values — it computes them
// from the projected cells, which is what makes "authored derived" detectable.
func Project(s *Schemas, p Profile) (Row, error) {
	vals := map[string]any{"agent_id": p.AgentID}
	for _, f := range mapping {
		info := s.Flat.Props[f.Col]
		v, ok := lookup(p.Answer, f.Path)
		if !ok {
			return Row{}, fmt.Errorf("%s: answer has no %s (for %s)", p.AgentID, f.Path, f.Col)
		}
		out, err := encode(info, f, v)
		if err != nil {
			return Row{}, fmt.Errorf("%s: %s: %w", p.AgentID, f.Col, err)
		}
		vals[f.Col] = out
	}
	vals["source_url"] = p.Provenance.SourceURL
	vals["checked_at"] = p.Provenance.CheckedAt
	vals["review_by"] = p.Provenance.ReviewBy
	vals["survey_method"] = p.Provenance.SurveyMethod
	vals["schema_version"] = p.Provenance.SchemaVersion
	d := Derive(cells(vals, s.Flat.Order))
	vals["mcp_ready"] = d.MCPReady
	vals["autonomous_ready"] = d.AutonomousReady
	vals["production_grade"] = d.ProductionGrade
	vals["strictness_score"] = d.StrictnessScore
	return Row{AgentID: p.AgentID, Values: vals}, nil
}

// Derived is the four metrics the tool computes and never authors.
type Derived struct {
	MCPReady        bool
	AutonomousReady bool
	ProductionGrade bool
	StrictnessScore int
}

// Derive computes the metrics from CSV cells. Keeping it a function of the
// *cells* (not of the answer) lets Check recompute them from models.csv on
// disk, so a hand-edited metric is caught even when nothing else changed.
func Derive(c map[string]string) Derived {
	strict := 0
	for _, col := range []string{"strict_typing", "stable_field_names", "stable_enums", "structured_errors"} {
		if c[col] == "true" {
			strict++
		}
	}
	vs := c["versioning_style"]
	if vs != "" && vs != "none" && vs != "null" {
		strict++
	}
	return Derived{
		MCPReady:        c["needs_mcp"] == "true" && c["preferred_interface"] == "mcp",
		AutonomousReady: c["auth_requires_human"] == "false" && c["machine_readable_spec"] == "true" && c["structured_errors"] == "true",
		ProductionGrade: c["backward_compatible"] == "true" && c["needs_retry_after"] == "true" &&
			c["needs_rate_limit_handling"] == "true" && c["standard_http_codes"] == "true",
		StrictnessScore: strict,
	}
}

// lookup walks a dotted path through the decoded answer.
func lookup(m map[string]any, path string) (any, bool) {
	var cur any = m
	for _, k := range strings.Split(path, ".") {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = mm[k]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// encode projects one nested value onto its flat column.
//
// null encodes as an empty string (absent) for a string column and as JSON
// null for a boolean column: "unknown" is expressible for a flag, and the
// issue's own rows spell an absent string as an empty cell.
func encode(info Prop, f field, v any) (any, error) {
	if v == nil {
		if info.Type == "boolean" {
			return nil, nil
		}
		if info.Nullable {
			return nil, nil
		}
		return "", nil
	}
	switch f.Kind {
	case kList:
		items, ok := v.([]any)
		if !ok {
			return nil, fmt.Errorf("want a list, got %T", v)
		}
		out := make([]string, 0, len(items))
		for _, it := range items {
			s, err := asString(it)
			if err != nil {
				return nil, err
			}
			out = append(out, s)
		}
		return strings.Join(out, "|"), nil
	case kIntList:
		items, ok := v.([]any)
		if !ok {
			return nil, fmt.Errorf("want a list, got %T", v)
		}
		out := make([]string, 0, len(items))
		for _, it := range items {
			n, err := asInt(it)
			if err != nil {
				return nil, err
			}
			out = append(out, strconv.Itoa(n))
		}
		return strings.Join(out, "|"), nil
	default:
		switch info.Type {
		case "boolean":
			b, ok := v.(bool)
			if !ok {
				return nil, fmt.Errorf("want a boolean, got %T", v)
			}
			return b, nil
		default:
			s, err := asString(v)
			if err != nil {
				return nil, err
			}
			return s, nil
		}
	}
}

func asString(v any) (string, error) {
	switch t := v.(type) {
	case string:
		return t, nil
	case bool:
		return strconv.FormatBool(t), nil
	case float64:
		return formatNum(t), nil
	case json.Number:
		return t.String(), nil
	default:
		return "", fmt.Errorf("want a string, got %T", v)
	}
}

func asInt(v any) (int, error) {
	switch t := v.(type) {
	case float64:
		if t != float64(int(t)) {
			return 0, fmt.Errorf("%v is not an integer", t)
		}
		return int(t), nil
	case json.Number:
		n, err := t.Int64()
		if err != nil {
			return 0, fmt.Errorf("%v is not an integer", t)
		}
		return int(n), nil
	case int:
		return t, nil
	default:
		return 0, fmt.Errorf("want an integer, got %T", v)
	}
}

func formatNum(f float64) string {
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// cells renders a row to CSV cells in the given column order. Booleans and
// integers are spelled the way the CSV spells them; null is "null".
func cells(vals map[string]any, order []string) map[string]string {
	out := make(map[string]string, len(order))
	for _, col := range order {
		out[col] = cell(vals[col])
	}
	return out
}

func cell(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case int:
		return strconv.Itoa(t)
	case json.Number:
		return t.String()
	default:
		return fmt.Sprint(t)
	}
}

// CSVOf renders a row's cells in the flat schema's column order.
func (r Row) CSVOf(s *Schemas) []string {
	out := make([]string, 0, len(s.Flat.Order))
	for _, col := range s.Flat.Order {
		out = append(out, cell(r.Values[col]))
	}
	return out
}
