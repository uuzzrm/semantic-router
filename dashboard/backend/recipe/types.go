package recipe

import (
	"context"
	"encoding/json"
)

const (
	MetadataSchemaVersion = "vllm-sr/recipe-metadata/v1"
	ProbeSchemaVersion    = "v1"
)

// Metadata is the stable, distribution-facing identity of one managed Recipe.
// Runtime routing, provider endpoints, credentials, and benchmark results do
// not belong in this document.
type Metadata struct {
	SchemaVersion string        `json:"schema_version" yaml:"schema_version"`
	ID            string        `json:"id" yaml:"id"`
	Name          string        `json:"name" yaml:"name"`
	Version       string        `json:"version" yaml:"version"`
	Description   string        `json:"description" yaml:"description"`
	Authors       []Author      `json:"authors" yaml:"authors"`
	License       string        `json:"license" yaml:"license"`
	Tags          []string      `json:"tags" yaml:"tags"`
	Links         MetadataLinks `json:"links" yaml:"links"`
}

type Author struct {
	Name  string  `json:"name" yaml:"name"`
	Email *string `json:"email,omitempty" yaml:"email,omitempty"`
	URL   *string `json:"url,omitempty" yaml:"url,omitempty"`
}

type MetadataLinks struct {
	Homepage      *string `json:"homepage,omitempty" yaml:"homepage,omitempty"`
	Source        string  `json:"source" yaml:"source"`
	Documentation *string `json:"documentation,omitempty" yaml:"documentation,omitempty"`
}

type FileHealth struct {
	Present   bool   `json:"present"`
	Digest    string `json:"digest,omitempty"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
}

type SourceHealth struct {
	Status string                `json:"status"`
	Files  map[string]FileHealth `json:"files"`
	Issues []string              `json:"issues"`
}

type Digests struct {
	Recipe   string `json:"recipe,omitempty"`
	Metadata string `json:"metadata,omitempty"`
	Config   string `json:"config,omitempty"`
	Probes   string `json:"probes,omitempty"`
	DSL      string `json:"dsl,omitempty"`
	README   string `json:"readme,omitempty"`
}

type Counts struct {
	UnifiedModels int `json:"unified_models"`
	Recipes       int `json:"recipes"`
	Decisions     int `json:"decisions"`
	Probes        int `json:"probes"`
}

type Descriptor struct {
	Managed      bool         `json:"managed"`
	Metadata     *Metadata    `json:"metadata,omitempty"`
	README       string       `json:"readme,omitempty"`
	SourceHealth SourceHealth `json:"source_health"`
	Digests      Digests      `json:"digests"`
	Counts       Counts       `json:"counts"`
}

type ExpectedAssertions struct {
	Decision         string              `json:"decision"`
	Recipe           string              `json:"recipe,omitempty"`
	Algorithm        string              `json:"algorithm,omitempty"`
	Alias            string              `json:"alias,omitempty"`
	Plugins          []string            `json:"plugins"`
	ForbiddenPlugins []string            `json:"forbidden_plugins"`
	PluginMatch      string              `json:"plugin_match"`
	Signals          map[string][]string `json:"signals"`
	ForbiddenSignals map[string][]string `json:"forbidden_signals"`
	SignalMatch      string              `json:"signal_match"`
}

type Padding struct {
	Text      string `json:"text" yaml:"text"`
	Repeat    int    `json:"repeat" yaml:"repeat"`
	Placement string `json:"placement" yaml:"placement"`
}

type ProbeSummary struct {
	ID            string             `json:"id"`
	DecisionID    string             `json:"decision_id"`
	VariantID     string             `json:"variant_id"`
	Model         string             `json:"model,omitempty"`
	QueryPreview  string             `json:"query_preview"`
	RequestShapes []string           `json:"request_shapes"`
	Tags          []string           `json:"tags"`
	Expected      ExpectedAssertions `json:"expected"`
	Editable      bool               `json:"editable"`
}

type ProbeDetail struct {
	ProbeSummary
	Query    string           `json:"query,omitempty"`
	Messages []map[string]any `json:"messages,omitempty"`
	Tools    []map[string]any `json:"tools,omitempty"`
	Repeat   int              `json:"repeat"`
	Padding  *Padding         `json:"padding,omitempty"`
	Notes    string           `json:"notes,omitempty"`
}

type Facets struct {
	Decisions map[string]int `json:"decisions"`
	Tags      map[string]int `json:"tags"`
	Models    map[string]int `json:"models"`
	Shapes    map[string]int `json:"shapes"`
}

type ProbeList struct {
	Items        []ProbeSummary `json:"items"`
	Page         int            `json:"page"`
	PageSize     int            `json:"page_size"`
	Total        int            `json:"total"`
	TotalPages   int            `json:"total_pages"`
	Facets       Facets         `json:"facets"`
	RecipeDigest string         `json:"recipe_digest"`
}

type ListOptions struct {
	Page     int
	PageSize int
	Query    string
	Decision string
	Tag      string
	Model    string
	Shape    string
}

type ChatRequest struct {
	Model    string           `json:"model,omitempty"`
	Messages []map[string]any `json:"messages"`
	Tools    []map[string]any `json:"tools,omitempty"`
}

type RunPlan struct {
	ProbeID      string           `json:"probe_id"`
	RecipeDigest string           `json:"recipe_digest"`
	Model        string           `json:"model,omitempty"`
	Messages     []map[string]any `json:"messages"`
	Tools        []map[string]any `json:"tools,omitempty"`
	Request      ChatRequest      `json:"request"`
	Editable     bool             `json:"editable"`
}

type EvalRequest struct {
	Text     string           `json:"text,omitempty"`
	Messages []map[string]any `json:"messages,omitempty"`
	Tools    []map[string]any `json:"tools,omitempty"`
	Model    string           `json:"model,omitempty"`
}

type Evaluator interface {
	Evaluate(context.Context, EvalRequest) (json.RawMessage, error)
}

type ActualOutcome struct {
	Decision          string              `json:"decision"`
	Model             string              `json:"model,omitempty"`
	Recipe            string              `json:"recipe,omitempty"`
	Algorithm         string              `json:"algorithm,omitempty"`
	Plugins           []string            `json:"plugins"`
	RecommendedModels []string            `json:"recommended_models"`
	MatchedSignals    map[string][]string `json:"matched_signals"`
	TraceDecisions    []string            `json:"trace_decisions"`
}

type ValidationChecks struct {
	Decision  bool `json:"decision"`
	Model     bool `json:"model"`
	Recipe    bool `json:"recipe"`
	Algorithm bool `json:"algorithm"`
	Plugins   bool `json:"plugins"`
	Signals   bool `json:"signals"`
	Alias     bool `json:"alias"`
	Trace     bool `json:"trace"`
}

type ValidationResult struct {
	ProbeID      string             `json:"probe_id"`
	RecipeDigest string             `json:"recipe_digest"`
	Passed       bool               `json:"passed"`
	Expected     ExpectedAssertions `json:"expected"`
	Actual       ActualOutcome      `json:"actual"`
	Checks       ValidationChecks   `json:"checks"`
	Failures     []string           `json:"failures"`
	LatencyMS    int64              `json:"latency_ms"`
	Error        string             `json:"error,omitempty"`
}
