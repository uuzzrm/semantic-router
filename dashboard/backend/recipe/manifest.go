package recipe

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

var (
	metadataIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	tagPattern        = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	semverPattern     = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)
	probeIDPattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
)

func decodeMetadata(data []byte) (Metadata, error) {
	if len(data) == 0 {
		return Metadata{}, errors.New("metadata.yaml is missing or empty")
	}
	var metadata Metadata
	if err := decodeStrictYAML(data, &metadata); err != nil {
		return Metadata{}, fmt.Errorf("metadata.yaml: %w", err)
	}
	if err := validateMetadata(metadata); err != nil {
		return metadata, fmt.Errorf("metadata.yaml: %w", err)
	}
	return metadata, nil
}

func validateMetadata(metadata Metadata) error {
	issues := []string{}
	if metadata.SchemaVersion != MetadataSchemaVersion {
		issues = append(issues, fmt.Sprintf("schema_version must be %q", MetadataSchemaVersion))
	}
	if !metadataIDPattern.MatchString(metadata.ID) {
		issues = append(issues, "id must be a lowercase kebab-case identifier")
	}
	if utf8.RuneCountInString(metadata.ID) > 64 {
		issues = append(issues, "id exceeds 64 characters")
	}
	if err := requireBoundedText("name", metadata.Name, 128); err != nil {
		issues = append(issues, err.Error())
	}
	if !semverPattern.MatchString(metadata.Version) || utf8.RuneCountInString(metadata.Version) > 128 {
		issues = append(issues, "version must be a semantic version")
	}
	if err := requireBoundedText("description", metadata.Description, 2048); err != nil {
		issues = append(issues, err.Error())
	}
	if len(metadata.Authors) == 0 {
		issues = append(issues, "authors must contain at least one author")
	}
	if len(metadata.Authors) > 32 {
		issues = append(issues, "authors must not contain more than 32 entries")
	}
	authorKeys := map[string]struct{}{}
	for index, author := range metadata.Authors {
		if err := requireBoundedText(fmt.Sprintf("authors[%d].name", index), author.Name, 128); err != nil {
			issues = append(issues, err.Error())
		}
		if author.Email != nil {
			address, err := mail.ParseAddress(*author.Email)
			if err != nil || address.Address != *author.Email {
				issues = append(issues, fmt.Sprintf("authors[%d].email must be a valid email address", index))
			}
		}
		if author.URL != nil && !isHTTPSURI(*author.URL) {
			issues = append(issues, fmt.Sprintf("authors[%d].url must be an HTTPS URI", index))
		}
		key := author.Name + "\x00" + optionalString(author.Email) + "\x00" + optionalString(author.URL)
		if _, duplicate := authorKeys[key]; duplicate {
			issues = append(issues, "authors must not contain duplicate entries")
		}
		authorKeys[key] = struct{}{}
	}
	if err := requireBoundedText("license", metadata.License, 128); err != nil {
		issues = append(issues, err.Error())
	}
	if len(metadata.Tags) == 0 {
		issues = append(issues, "tags must contain at least one tag")
	}
	if len(metadata.Tags) > 32 {
		issues = append(issues, "tags must not contain more than 32 entries")
	}
	tagSet := map[string]struct{}{}
	for _, tag := range metadata.Tags {
		if !tagPattern.MatchString(tag) || utf8.RuneCountInString(tag) > 64 {
			issues = append(issues, fmt.Sprintf("tag %q must be lowercase kebab-case", tag))
		}
		if _, duplicate := tagSet[tag]; duplicate {
			issues = append(issues, "tags must not contain duplicates")
		}
		tagSet[tag] = struct{}{}
	}
	if !isHTTPSURI(metadata.Links.Source) {
		issues = append(issues, "links.source must be an HTTPS URI")
	}
	if metadata.Links.Homepage != nil && !isHTTPSURI(*metadata.Links.Homepage) {
		issues = append(issues, "links.homepage must be an HTTPS URI")
	}
	if metadata.Links.Documentation != nil && !isHTTPSURI(*metadata.Links.Documentation) {
		issues = append(issues, "links.documentation must be an HTTPS URI")
	}
	if len(issues) > 0 {
		return errors.New(strings.Join(stableUnique(issues), "; "))
	}
	return nil
}

func requireBoundedText(name, value string, maxRunes int) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("%s must not be empty", name)
	}
	if utf8.RuneCountInString(value) > maxRunes {
		return fmt.Errorf("%s exceeds %d characters", name, maxRunes)
	}
	return nil
}

func isHTTPSURI(raw string) bool {
	if !strings.HasPrefix(raw, "https://") {
		return false
	}
	parsed, err := url.Parse(raw)
	return err == nil && parsed.Scheme == "https" && parsed.Host != ""
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

type probeManifest struct {
	SchemaVersion      string             `yaml:"schema_version"`
	Name               string             `yaml:"name"`
	Description        string             `yaml:"description"`
	RoutingAssets      routingAssets      `yaml:"routing_assets"`
	RouterEvalEndpoint string             `yaml:"router_eval_endpoint"`
	Evaluation         evaluationSettings `yaml:"evaluation"`
	Acceptance         acceptancePolicy   `yaml:"acceptance"`
	Coverage           *coveragePolicy    `yaml:"coverage"`
	Decisions          []probeDecision    `yaml:"decisions"`
}

type routingAssets struct {
	YAML string `yaml:"yaml"`
	DSL  string `yaml:"dsl"`
}

type evaluationSettings struct {
	RequestTimeoutSeconds *float64 `yaml:"request_timeout_seconds"`
	Concurrency           *int     `yaml:"concurrency"`
}

type acceptancePolicy struct {
	MinProbePassRate    float64 `yaml:"min_probe_pass_rate"`
	MinDecisionPassRate float64 `yaml:"min_decision_pass_rate"`
}

type coveragePolicy struct {
	MinSignalAssertionPercent     float64            `yaml:"min_signal_assertion_percent"`
	MinProjectionAssertionPercent float64            `yaml:"min_projection_assertion_percent"`
	MinAlgorithmAssertionPercent  float64            `yaml:"min_algorithm_assertion_percent"`
	MinPluginAssertionPercent     float64            `yaml:"min_plugin_assertion_percent"`
	RequiredRequestShapes         []string           `yaml:"required_request_shapes"`
	MinTagCounts                  map[string]int     `yaml:"min_tag_counts"`
	MinTagPassRate                map[string]float64 `yaml:"min_tag_pass_rate"`
}

type robustnessPolicy struct {
	MinPassRate float64 `yaml:"min_pass_rate"`
}

type probeDecision struct {
	ID                string              `yaml:"id"`
	ExpectedDecision  string              `yaml:"expected_decision"`
	Model             string              `yaml:"model"`
	ExpectedRecipe    string              `yaml:"expected_recipe"`
	ExpectedAlgorithm string              `yaml:"expected_algorithm"`
	ExpectedPlugins   []string            `yaml:"expected_plugins"`
	ForbiddenPlugins  []string            `yaml:"forbidden_plugins"`
	PluginMatch       string              `yaml:"plugin_match"`
	ExpectedAlias     string              `yaml:"expected_alias"`
	ExpectedSignals   map[string][]string `yaml:"expected_signals"`
	ForbiddenSignals  map[string][]string `yaml:"forbidden_signals"`
	SignalMatch       string              `yaml:"signal_match"`
	Robustness        robustnessPolicy    `yaml:"robustness"`
	Objective         string              `yaml:"objective"`
	Notes             string              `yaml:"notes"`
	Variants          []probeVariant      `yaml:"variants"`
}

type probeVariant struct {
	ID              string              `yaml:"id"`
	Query           string              `yaml:"query"`
	Messages        []map[string]any    `yaml:"messages"`
	Tools           []map[string]any    `yaml:"tools"`
	Repeat          int                 `yaml:"repeat"`
	Padding         *Padding            `yaml:"padding"`
	Tags            []string            `yaml:"tags"`
	Notes           string              `yaml:"notes"`
	ExpectedSignals map[string][]string `yaml:"expected_signals"`
}

func decodeProbes(data []byte) (probeManifest, error) {
	if len(data) == 0 {
		return probeManifest{}, errors.New("probes.yaml is missing or empty")
	}
	var manifest probeManifest
	if err := decodeStrictYAML(data, &manifest); err != nil {
		return probeManifest{}, fmt.Errorf("probes.yaml: %w", err)
	}
	if err := validateProbeManifest(&manifest); err != nil {
		return manifest, fmt.Errorf("probes.yaml: %w", err)
	}
	return manifest, nil
}

func validateProbeManifest(manifest *probeManifest) error {
	issues := []string{}
	validateProbeManifestHeader(manifest, &issues)
	decisionIDs := map[string]struct{}{}
	probeIDs := map[string]struct{}{}
	for decisionIndex := range manifest.Decisions {
		validateProbeDecision(
			&manifest.Decisions[decisionIndex],
			decisionIndex,
			decisionIDs,
			probeIDs,
			&issues,
		)
	}
	if len(issues) > 0 {
		return errors.New(strings.Join(stableUnique(issues), "; "))
	}
	return nil
}

func validateProbeManifestHeader(manifest *probeManifest, issues *[]string) {
	if manifest.SchemaVersion != ProbeSchemaVersion {
		*issues = append(*issues, fmt.Sprintf("schema_version must be %q", ProbeSchemaVersion))
	}
	if err := requireBoundedText("name", manifest.Name, 200); err != nil {
		*issues = append(*issues, err.Error())
	}
	if strings.TrimSpace(manifest.RoutingAssets.YAML) == "" || strings.TrimSpace(manifest.RoutingAssets.DSL) == "" {
		*issues = append(*issues, "routing_assets.yaml and routing_assets.dsl are required")
	}
	if manifest.Coverage == nil {
		*issues = append(*issues, "coverage is required")
	}
	if len(manifest.Decisions) == 0 {
		*issues = append(*issues, "decisions must contain at least one decision")
	}
	if timeout := manifest.Evaluation.RequestTimeoutSeconds; timeout != nil && (*timeout < 1 || *timeout > 1200) {
		*issues = append(*issues, "evaluation.request_timeout_seconds must be between 1 and 1200")
	}
	if concurrency := manifest.Evaluation.Concurrency; concurrency != nil && (*concurrency < 1 || *concurrency > 64) {
		*issues = append(*issues, "evaluation.concurrency must be between 1 and 64")
	}
}

func validateProbeDecision(
	decision *probeDecision,
	decisionIndex int,
	decisionIDs map[string]struct{},
	probeIDs map[string]struct{},
	issues *[]string,
) {
	label := fmt.Sprintf("decisions[%d]", decisionIndex)
	if !validProbeIdentifier(decision.ID) {
		*issues = append(*issues, label+".id is invalid")
	}
	if _, duplicate := decisionIDs[decision.ID]; duplicate {
		*issues = append(*issues, fmt.Sprintf("duplicate decision id %q", decision.ID))
	}
	decisionIDs[decision.ID] = struct{}{}
	if strings.TrimSpace(decision.ExpectedDecision) == "" {
		*issues = append(*issues, label+".expected_decision is required")
	}
	decision.PluginMatch = defaultMatchMode(decision.PluginMatch)
	decision.SignalMatch = defaultMatchMode(decision.SignalMatch)
	if !validMatchMode(decision.PluginMatch) {
		*issues = append(*issues, label+".plugin_match must be contains or exact")
	}
	if !validMatchMode(decision.SignalMatch) {
		*issues = append(*issues, label+".signal_match must be contains or exact")
	}
	validateUniqueStrings(issues, label+".expected_plugins", decision.ExpectedPlugins)
	validateUniqueStrings(issues, label+".forbidden_plugins", decision.ForbiddenPlugins)
	validateSignalMap(issues, label+".expected_signals", decision.ExpectedSignals)
	validateSignalMap(issues, label+".forbidden_signals", decision.ForbiddenSignals)
	if len(decision.Variants) == 0 {
		*issues = append(*issues, label+".variants must contain at least one variant")
	}
	for variantIndex := range decision.Variants {
		validateProbeVariant(decision.ID, &decision.Variants[variantIndex], label, variantIndex, probeIDs, issues)
	}
}

func validateProbeVariant(
	decisionID string,
	variant *probeVariant,
	decisionLabel string,
	variantIndex int,
	probeIDs map[string]struct{},
	issues *[]string,
) {
	label := fmt.Sprintf("%s.variants[%d]", decisionLabel, variantIndex)
	if !validProbeIdentifier(variant.ID) {
		*issues = append(*issues, label+".id is invalid")
	}
	id := probeKey(decisionID, variant.ID)
	if _, duplicate := probeIDs[id]; duplicate {
		*issues = append(*issues, fmt.Sprintf("duplicate probe id %s:%s", decisionID, variant.ID))
	}
	probeIDs[id] = struct{}{}
	variant.Query = strings.TrimSpace(variant.Query)
	if (variant.Query != "") == (len(variant.Messages) > 0) {
		*issues = append(*issues, label+" must contain exactly one of query or messages")
	}
	if variant.Repeat == 0 {
		variant.Repeat = 1
	}
	if variant.Repeat < 1 || variant.Repeat > 10_000 {
		*issues = append(*issues, label+".repeat must be between 1 and 10000")
	}
	if variant.Padding != nil {
		validateProbePadding(variant.Padding, label, issues)
	}
	validateUniqueStrings(issues, label+".tags", variant.Tags)
	validateSignalMap(issues, label+".expected_signals", variant.ExpectedSignals)
	if err := validateMessageObjects(variant.Messages); err != nil {
		*issues = append(*issues, label+".messages: "+err.Error())
	}
	if err := validateJSONObjects(variant.Tools); err != nil {
		*issues = append(*issues, label+".tools: "+err.Error())
	}
}

func validateProbePadding(padding *Padding, variantLabel string, issues *[]string) {
	padding.Text = strings.TrimSpace(padding.Text)
	if padding.Text == "" {
		*issues = append(*issues, variantLabel+".padding.text is required")
	}
	if padding.Repeat == 0 {
		padding.Repeat = 1
	}
	if padding.Repeat < 1 || padding.Repeat > 10_000 {
		*issues = append(*issues, variantLabel+".padding.repeat must be between 1 and 10000")
	}
	padding.Placement = strings.ToLower(strings.TrimSpace(padding.Placement))
	if padding.Placement == "" {
		padding.Placement = "before"
	}
	if padding.Placement != "before" && padding.Placement != "after" && padding.Placement != "around" {
		*issues = append(*issues, variantLabel+".padding.placement must be before, after, or around")
	}
}

func validProbeIdentifier(value string) bool {
	return len(value) <= 128 && probeIDPattern.MatchString(value)
}

func defaultMatchMode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "contains"
	}
	return value
}

func validMatchMode(value string) bool {
	return value == "contains" || value == "exact"
}

func validateUniqueStrings(issues *[]string, label string, values []string) {
	seen := map[string]struct{}{}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			*issues = append(*issues, label+" must not contain empty values")
		}
		if _, duplicate := seen[value]; duplicate {
			*issues = append(*issues, label+" must not contain duplicates")
		}
		seen[value] = struct{}{}
	}
}

func validateSignalMap(issues *[]string, label string, signals map[string][]string) {
	for signalType, names := range signals {
		if strings.TrimSpace(signalType) == "" || len(names) == 0 {
			*issues = append(*issues, label+" must use non-empty types and value lists")
		}
		validateUniqueStrings(issues, label+"."+signalType, names)
	}
}

func validateMessageObjects(messages []map[string]any) error {
	return validateJSONObjects(messages)
}

func validateJSONObjects(objects []map[string]any) error {
	for index, object := range objects {
		if _, err := json.Marshal(object); err != nil {
			return fmt.Errorf("item %d is not JSON-compatible: %w", index, err)
		}
	}
	return nil
}

func flattenProbes(manifest probeManifest) ([]ProbeDetail, map[string]ProbeDetail) {
	probes := make([]ProbeDetail, 0)
	byID := make(map[string]ProbeDetail)
	for _, decision := range manifest.Decisions {
		expected := ExpectedAssertions{
			Decision:         decision.ExpectedDecision,
			Recipe:           decision.ExpectedRecipe,
			Algorithm:        decision.ExpectedAlgorithm,
			Alias:            decision.ExpectedAlias,
			Plugins:          nonNilStrings(decision.ExpectedPlugins),
			ForbiddenPlugins: nonNilStrings(decision.ForbiddenPlugins),
			PluginMatch:      decision.PluginMatch,
			Signals:          nonNilSignalMap(decision.ExpectedSignals),
			ForbiddenSignals: nonNilSignalMap(decision.ForbiddenSignals),
			SignalMatch:      decision.SignalMatch,
		}
		for _, variant := range decision.Variants {
			variantExpected := expected
			if variant.ExpectedSignals != nil {
				variantExpected.Signals = nonNilSignalMap(variant.ExpectedSignals)
			}
			shapes := []string{"text"}
			preview := strings.TrimSpace(variant.Query)
			if len(variant.Messages) > 0 {
				shapes = []string{"messages"}
				preview = summarizeMessages(variant.Messages)
			}
			if len(variant.Tools) > 0 {
				shapes = append(shapes, "tools")
			}
			notes := strings.TrimSpace(variant.Notes)
			if notes == "" {
				notes = strings.TrimSpace(decision.Notes)
			}
			if notes == "" {
				notes = strings.TrimSpace(decision.Objective)
			}
			probe := ProbeDetail{
				ProbeSummary: ProbeSummary{
					ID:            decision.ID + ":" + variant.ID,
					DecisionID:    decision.ID,
					VariantID:     variant.ID,
					Model:         decision.Model,
					QueryPreview:  truncateRunes(preview, 240),
					RequestShapes: shapes,
					Tags:          nonNilStrings(variant.Tags),
					Expected:      variantExpected,
					Editable:      probeVariantIsEditable(variant),
				},
				Query:    variant.Query,
				Messages: cloneObjects(variant.Messages),
				Tools:    cloneObjects(variant.Tools),
				Repeat:   variant.Repeat,
				Padding:  variant.Padding,
				Notes:    notes,
			}
			probes = append(probes, probe)
			byID[probeKey(decision.ID, variant.ID)] = probe
		}
	}
	return probes, byID
}

func probeVariantIsEditable(variant probeVariant) bool {
	if strings.TrimSpace(variant.Query) != "" {
		return true
	}
	if len(variant.Messages) == 0 {
		return false
	}
	role, _ := variant.Messages[len(variant.Messages)-1]["role"].(string)
	return strings.EqualFold(strings.TrimSpace(role), "user")
}

func summarizeMessages(messages []map[string]any) string {
	for index := len(messages) - 1; index >= 0; index-- {
		role, _ := messages[index]["role"].(string)
		if strings.ToLower(strings.TrimSpace(role)) != "user" {
			continue
		}
		if content := summarizeContent(messages[index]["content"]); content != "" {
			return content
		}
	}
	data, _ := json.Marshal(messages)
	return string(data)
}

func summarizeContent(content any) string {
	if text, ok := content.(string); ok {
		return strings.TrimSpace(text)
	}
	items, ok := content.([]any)
	if !ok {
		return ""
	}
	parts := []string{}
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		kind, _ := item["type"].(string)
		if kind != "" && kind != "text" && kind != "input_text" {
			continue
		}
		text, _ := item["text"].(string)
		if strings.TrimSpace(text) != "" {
			parts = append(parts, strings.TrimSpace(text))
		}
	}
	return strings.Join(parts, " ")
}

func truncateRunes(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max-1]) + "…"
}

func nonNilStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return append([]string(nil), values...)
}

func nonNilSignalMap(values map[string][]string) map[string][]string {
	result := make(map[string][]string, len(values))
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		result[key] = nonNilStrings(values[key])
	}
	return result
}

type configDocument struct {
	Global struct {
		Router configRouterDocument `yaml:"router"`
	} `yaml:"global"`
	Routing struct {
		Decisions []struct {
			Name string `yaml:"name"`
		} `yaml:"decisions"`
	} `yaml:"routing"`
	Entrypoints []struct {
		ModelNames []string `yaml:"model_names"`
		Recipe     string   `yaml:"recipe"`
	} `yaml:"entrypoints"`
	Recipes []struct {
		Name    string `yaml:"name"`
		Routing struct {
			Decisions []struct {
				Name string `yaml:"name"`
			} `yaml:"decisions"`
		} `yaml:"routing"`
	} `yaml:"recipes"`
}

type configRouterDocument struct {
	AutoModelName  string    `yaml:"auto_model_name"`
	AutoModelNames yaml.Node `yaml:"auto_model_names"`
}

type configProjection struct {
	counts          Counts
	autoModels      []string
	modelsByRecipe  map[string][]string
	unifiedModelIDs []string
}

func projectConfig(data []byte) (configProjection, error) {
	if len(data) == 0 {
		return configProjection{}, errors.New("missing or empty")
	}
	var config configDocument
	// Config is an independently versioned runtime contract, so intentionally
	// do not use KnownFields here. Only the count projection above is consumed.
	if err := decodeYAML(data, &config); err != nil {
		return configProjection{}, err
	}
	projection := configProjection{modelsByRecipe: map[string][]string{}}
	autoModels, err := projectAutoModels(config.Global.Router)
	if err != nil {
		return configProjection{}, err
	}
	projection.autoModels = autoModels

	models := map[string]struct{}{}
	for _, model := range projection.autoModels {
		models[model] = struct{}{}
	}
	for _, entrypoint := range config.Entrypoints {
		recipeName := strings.TrimSpace(entrypoint.Recipe)
		for _, model := range entrypoint.ModelNames {
			if model = strings.TrimSpace(model); model != "" {
				models[model] = struct{}{}
				projection.modelsByRecipe[recipeName] = append(projection.modelsByRecipe[recipeName], model)
			}
		}
		projection.modelsByRecipe[recipeName] = stableUnique(projection.modelsByRecipe[recipeName])
	}
	counts := Counts{UnifiedModels: len(models), Recipes: len(config.Recipes), Decisions: len(config.Routing.Decisions)}
	for _, recipe := range config.Recipes {
		counts.Decisions += len(recipe.Routing.Decisions)
	}
	if counts.Recipes == 0 {
		counts.Recipes = 1
	}
	projection.counts = counts
	projection.unifiedModelIDs = make([]string, 0, len(models))
	for model := range models {
		projection.unifiedModelIDs = append(projection.unifiedModelIDs, model)
	}
	sort.Strings(projection.unifiedModelIDs)
	return projection, nil
}

func projectAutoModels(router configRouterDocument) ([]string, error) {
	// Presence is part of the compatibility contract: an explicit list, even an
	// empty one, replaces both the legacy field and historical defaults.
	if router.AutoModelNames.Kind != 0 {
		var explicit []string
		if err := router.AutoModelNames.Decode(&explicit); err != nil {
			return nil, fmt.Errorf("global.router.auto_model_names: %w", err)
		}
		return stableUnique(cleanStrings(explicit)), nil
	}

	configured := strings.TrimSpace(router.AutoModelName)
	if configured == "" {
		configured = "MoM"
	}
	return stableUnique([]string{"vllm-sr/auto", "auto", configured}), nil
}

func (projection configProjection) requestModelFor(expectedRecipe string) (string, error) {
	if candidates := projection.modelsByRecipe[strings.TrimSpace(expectedRecipe)]; len(candidates) > 0 {
		return candidates[0], nil
	}
	if len(projection.autoModels) > 0 {
		return projection.autoModels[0], nil
	}
	if len(projection.unifiedModelIDs) == 1 {
		return projection.unifiedModelIDs[0], nil
	}
	return "", errors.New("config has no unambiguous request-facing model")
}

func decodeYAML(data []byte, target any) error {
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("must contain exactly one YAML document")
		}
		return err
	}
	return nil
}
