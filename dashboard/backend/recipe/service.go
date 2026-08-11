package recipe

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

const (
	defaultPageSize                 = 50
	maxPageSize                     = 200
	maxSearchLength                 = 256
	maxMetadataBytes          int64 = 256 << 10
	maxConfigBytes            int64 = 16 << 20
	maxProbesBytes            int64 = 8 << 20
	maxDSLBytes               int64 = 8 << 20
	maxREADMEBytes            int64 = 2 << 20
	maxRequestBytes                 = 2 << 20
	maxEvalResponseBytes            = 4 << 20
	defaultEvalTimeoutSeconds       = 60.0
)

var (
	ErrUnmanaged     = errors.New("active configuration is not a managed recipe")
	ErrProbeNotFound = errors.New("probe not found")
	ErrInvalid       = errors.New("managed recipe is invalid")
	ErrBadRequest    = errors.New("invalid recipe request")
	ErrStale         = errors.New("managed recipe changed; refresh probes")
	ErrUpstream      = errors.New("router eval request failed")
)

var recipeFiles = []struct {
	key      string
	name     string
	maxBytes int64
}{
	{key: "metadata", name: "metadata.yaml", maxBytes: maxMetadataBytes},
	{key: "config", name: "config.yaml", maxBytes: maxConfigBytes},
	{key: "probes", name: "probes.yaml", maxBytes: maxProbesBytes},
	{key: "dsl", name: "recipe.dsl", maxBytes: maxDSLBytes},
	{key: "readme", name: "README.md", maxBytes: maxREADMEBytes},
}

type Options struct {
	Directory    string
	RouterAPIURL string
	HTTPClient   *http.Client
	Evaluator    Evaluator
}

// Service owns the single active Recipe selected when the dashboard process is
// constructed. It only reads the five fixed filenames in Directory and never
// scans or resolves a catalog of sibling directories.
type Service struct {
	directory string
	evaluator Evaluator
}

func NewService(options Options) *Service {
	evaluator := options.Evaluator
	if evaluator == nil {
		client := options.HTTPClient
		if client == nil {
			client = &http.Client{
				CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
					return http.ErrUseLastResponse
				},
			}
		}
		evaluator = &HTTPRouterEvaluator{
			BaseURL: strings.TrimSpace(options.RouterAPIURL),
			Client:  client,
		}
	}
	return &Service{
		directory: filepath.Clean(options.Directory),
		evaluator: evaluator,
	}
}

func (s *Service) Describe() (Descriptor, error) {
	snapshot, descriptor, err := s.loadSnapshot()
	if errors.Is(err, ErrUnmanaged) {
		return descriptor, nil
	}
	if err != nil {
		return descriptor, err
	}

	descriptor.Metadata = &snapshot.metadata
	descriptor.README = string(snapshot.files["readme"])
	descriptor.Counts = snapshot.counts
	return descriptor, nil
}

func (s *Service) ListProbes(options ListOptions) (ProbeList, error) {
	snapshot, _, err := s.loadSnapshot()
	if err != nil {
		return ProbeList{}, err
	}
	if err := normalizeListOptions(&options); err != nil {
		return ProbeList{}, err
	}

	filtered := make([]ProbeDetail, 0, len(snapshot.probes))
	for _, probe := range snapshot.probes {
		if matchesFilters(probe, options) {
			filtered = append(filtered, probe)
		}
	}

	total := len(filtered)
	totalPages := 0
	if total > 0 {
		totalPages = (total + options.PageSize - 1) / options.PageSize
	}
	start := total
	pageIndex := options.Page - 1
	// Compare before multiplying so an authenticated caller cannot overflow an
	// int with an arbitrarily large page query and panic the Dashboard process.
	if pageIndex <= total/options.PageSize {
		start = pageIndex * options.PageSize
	}
	end := start + options.PageSize
	if end > total {
		end = total
	}

	items := make([]ProbeSummary, 0, end-start)
	for _, probe := range filtered[start:end] {
		items = append(items, probe.ProbeSummary)
	}
	return ProbeList{
		Items:        items,
		Page:         options.Page,
		PageSize:     options.PageSize,
		Total:        total,
		TotalPages:   totalPages,
		Facets:       buildFacets(snapshot.probes),
		RecipeDigest: snapshot.recipeDigest,
	}, nil
}

func (s *Service) Probe(decisionID, variantID string) (ProbeDetail, string, error) {
	snapshot, _, err := s.loadSnapshot()
	if err != nil {
		return ProbeDetail{}, "", err
	}
	probe, ok := snapshot.byID[probeKey(decisionID, variantID)]
	if !ok {
		return ProbeDetail{}, snapshot.recipeDigest, ErrProbeNotFound
	}
	return probe, snapshot.recipeDigest, nil
}

func (s *Service) RunPlan(decisionID, variantID string, expectedDigest ...string) (RunPlan, error) {
	snapshot, _, err := s.loadSnapshot()
	if err != nil {
		return RunPlan{}, err
	}
	if digestErr := requireExpectedDigest(snapshot.recipeDigest, expectedDigest); digestErr != nil {
		return RunPlan{}, digestErr
	}
	probe, ok := snapshot.byID[probeKey(decisionID, variantID)]
	if !ok {
		return RunPlan{}, ErrProbeNotFound
	}
	request, err := materializeChatRequest(probe)
	if err != nil {
		return RunPlan{}, err
	}
	return RunPlan{
		ProbeID:      probe.ID,
		RecipeDigest: snapshot.recipeDigest,
		Model:        request.Model,
		Messages:     request.Messages,
		Tools:        request.Tools,
		Request:      request,
		Editable:     probe.Editable,
	}, nil
}

func (s *Service) Validate(ctx context.Context, decisionID, variantID string, expectedDigest ...string) (ValidationResult, error) {
	started := time.Now()
	snapshot, _, err := s.loadSnapshot()
	if err != nil {
		return ValidationResult{}, err
	}
	if digestErr := requireExpectedDigest(snapshot.recipeDigest, expectedDigest); digestErr != nil {
		return ValidationResult{}, digestErr
	}
	probe, ok := snapshot.byID[probeKey(decisionID, variantID)]
	if !ok {
		return ValidationResult{}, ErrProbeNotFound
	}

	result := ValidationResult{
		ProbeID:      probe.ID,
		RecipeDigest: snapshot.recipeDigest,
		Expected:     probe.Expected,
		Actual: ActualOutcome{
			Plugins:           []string{},
			RecommendedModels: []string{},
			MatchedSignals:    map[string][]string{},
			TraceDecisions:    []string{},
		},
		Failures: []string{},
	}
	evalRequest, err := materializeEvalRequest(probe)
	if err != nil {
		return ValidationResult{}, err
	}
	evalContext, cancel := context.WithTimeout(ctx, snapshot.requestTimeout)
	defer cancel()
	raw, err := s.evaluator.Evaluate(evalContext, evalRequest)
	result.LatencyMS = time.Since(started).Milliseconds()
	if err != nil {
		result.Error = err.Error()
		result.Failures = append(result.Failures, "router eval request failed")
		return result, fmt.Errorf("%w: %w", ErrUpstream, err)
	}

	actual, checks, failures, err := compareEvalResponse(raw, probe, snapshot.probes)
	if err != nil {
		result.Error = err.Error()
		result.Failures = append(result.Failures, "router eval response was invalid")
		return result, fmt.Errorf("%w: %w", ErrUpstream, err)
	}
	result.Actual = actual
	result.Checks = checks
	result.Failures = failures
	result.Passed = len(failures) == 0
	return result, nil
}

func requireExpectedDigest(actual string, expected []string) error {
	if len(expected) == 0 {
		return nil
	}
	candidate := strings.TrimSpace(expected[0])
	if candidate == "" {
		return fmt.Errorf("%w: expected recipe digest must not be empty", ErrBadRequest)
	}
	if candidate != actual {
		return ErrStale
	}
	return nil
}

type snapshot struct {
	metadata       Metadata
	files          map[string][]byte
	probes         []ProbeDetail
	byID           map[string]ProbeDetail
	recipeDigest   string
	counts         Counts
	requestTimeout time.Duration
}

func (s *Service) loadSnapshot() (*snapshot, Descriptor, error) {
	files, health, digests, issues := s.readFixedFiles()
	descriptor := Descriptor{
		Managed: false,
		SourceHealth: SourceHealth{
			Status: "unmanaged",
			Files:  health,
			Issues: []string{},
		},
		Digests: digests,
		Counts:  Counts{},
	}
	if !health["metadata"].Present {
		if len(issues) > 0 {
			descriptor.SourceHealth.Issues = issues
		}
		return nil, descriptor, ErrUnmanaged
	}

	descriptor.Managed = true
	for _, file := range recipeFiles {
		if !health[file.key].Present {
			issues = append(issues, fmt.Sprintf("required file %s is missing", file.name))
		}
	}

	metadata, err := decodeMetadata(files["metadata"])
	if err != nil {
		issues = append(issues, err.Error())
	}
	probes, err := decodeProbes(files["probes"])
	if err != nil {
		issues = append(issues, err.Error())
	}
	configProjection, err := projectConfig(files["config"])
	if err != nil {
		issues = append(issues, fmt.Sprintf("config.yaml: %v", err))
	}
	if metadata.ID != "" && probes.Name != "" && metadata.ID != probes.Name {
		issues = append(issues, fmt.Sprintf("metadata id %q does not match probes name %q", metadata.ID, probes.Name))
	}

	if len(issues) > 0 {
		descriptor.Metadata = metadataIfPresent(metadata)
		descriptor.SourceHealth.Status = "invalid"
		descriptor.SourceHealth.Issues = stableUnique(issues)
		return nil, descriptor, fmt.Errorf("%w: %s", ErrInvalid, strings.Join(descriptor.SourceHealth.Issues, "; "))
	}

	flattened, byID := flattenProbes(probes)
	defaultProbeIndex := 0
	for index := range flattened {
		probe := flattened[index]
		if probe.Model == "" {
			var model string
			var resolveErr error
			expectedRecipe := normalizeExpectedRecipe(probe.Expected.Recipe)
			if expectedRecipe == "default" && len(configProjection.autoModels) > 0 {
				// Match the offline harness: only model-less probes in the default
				// recipe participate in auto-entrypoint round-robin assignment.
				model = configProjection.autoModels[defaultProbeIndex%len(configProjection.autoModels)]
				defaultProbeIndex++
			} else {
				// Dashboard Run requires an executable model. For named recipes,
				// preserve the existing policy of selecting their first entrypoint.
				model, resolveErr = configProjection.requestModelFor(expectedRecipe)
			}
			if resolveErr != nil {
				descriptor.SourceHealth.Status = "invalid"
				descriptor.SourceHealth.Issues = []string{fmt.Sprintf("probe %s: %v", probe.ID, resolveErr)}
				return nil, descriptor, fmt.Errorf("%w: %s", ErrInvalid, descriptor.SourceHealth.Issues[0])
			}
			probe.Model = model
			flattened[index] = probe
			byID[probeKey(probe.DecisionID, probe.VariantID)] = probe
		}
	}
	recipeDigest := digestRecipe(files)
	digests.Recipe = recipeDigest
	descriptor.Digests = digests
	descriptor.SourceHealth.Status = "ready"
	descriptor.Metadata = &metadata
	descriptor.README = string(files["readme"])
	counts := configProjection.counts
	counts.Probes = len(flattened)
	descriptor.Counts = counts
	requestTimeoutSeconds := defaultEvalTimeoutSeconds
	if probes.Evaluation.RequestTimeoutSeconds != nil {
		requestTimeoutSeconds = *probes.Evaluation.RequestTimeoutSeconds
	}
	return &snapshot{
		metadata:       metadata,
		files:          files,
		probes:         flattened,
		byID:           byID,
		recipeDigest:   recipeDigest,
		counts:         counts,
		requestTimeout: time.Duration(requestTimeoutSeconds * float64(time.Second)),
	}, descriptor, nil
}

func (s *Service) readFixedFiles() (map[string][]byte, map[string]FileHealth, Digests, []string) {
	files := make(map[string][]byte, len(recipeFiles))
	health := make(map[string]FileHealth, len(recipeFiles))
	digests := Digests{}
	issues := []string{}
	for _, file := range recipeFiles {
		data, err := readBoundedFile(filepath.Join(s.directory, file.name), file.maxBytes)
		if errors.Is(err, os.ErrNotExist) {
			health[file.key] = FileHealth{Present: false}
			continue
		}
		if err != nil {
			health[file.key] = FileHealth{Present: true}
			issues = append(issues, fmt.Sprintf("%s: %s", file.name, safeFileError(err)))
			continue
		}
		digest := digestBytes(data)
		files[file.key] = data
		health[file.key] = FileHealth{Present: true, Digest: digest, SizeBytes: int64(len(data))}
		switch file.key {
		case "metadata":
			digests.Metadata = digest
		case "config":
			digests.Config = digest
		case "probes":
			digests.Probes = digest
		case "dsl":
			digests.DSL = digest
		case "readme":
			digests.README = digest
		}
	}
	return files, health, digests, issues
}

func safeFileError(err error) string {
	message := err.Error()
	if errors.Is(err, os.ErrPermission) {
		return "cannot be read"
	}
	if strings.Contains(message, "exceeds ") || strings.Contains(message, "must be ") || strings.Contains(message, "must not be ") {
		return message
	}
	return "cannot be read"
}

func readBoundedFile(path string, maxBytes int64) ([]byte, error) {
	pathInfo, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if pathInfo.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("must not be a symbolic link")
	}
	if !pathInfo.Mode().IsRegular() {
		return nil, errors.New("must be a regular file")
	}
	if pathInfo.Size() > maxBytes {
		return nil, fmt.Errorf("exceeds %d byte limit", maxBytes)
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || !os.SameFile(pathInfo, info) {
		return nil, errors.New("must be a regular file")
	}
	if info.Size() > maxBytes {
		return nil, fmt.Errorf("exceeds %d byte limit", maxBytes)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("exceeds %d byte limit", maxBytes)
	}
	if !utf8.Valid(data) {
		return nil, errors.New("must be valid UTF-8")
	}
	return data, nil
}

func digestBytes(data []byte) string {
	digest := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func digestRecipe(files map[string][]byte) string {
	hash := sha256.New()
	for _, file := range recipeFiles {
		_, _ = io.WriteString(hash, file.name)
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write(files[file.key])
		_, _ = hash.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}

func metadataIfPresent(metadata Metadata) *Metadata {
	if metadata.SchemaVersion == "" && metadata.ID == "" && metadata.Name == "" {
		return nil
	}
	return &metadata
}

func normalizeListOptions(options *ListOptions) error {
	if options.Page == 0 {
		options.Page = 1
	}
	if options.Page < 1 {
		return fmt.Errorf("%w: page must be at least 1", ErrBadRequest)
	}
	if options.PageSize == 0 {
		options.PageSize = defaultPageSize
	}
	if options.PageSize < 1 || options.PageSize > maxPageSize {
		return fmt.Errorf("%w: page_size must be between 1 and %d", ErrBadRequest, maxPageSize)
	}
	options.Query = strings.TrimSpace(options.Query)
	if utf8.RuneCountInString(options.Query) > maxSearchLength {
		return fmt.Errorf("%w: q exceeds %d characters", ErrBadRequest, maxSearchLength)
	}
	options.Decision = strings.TrimSpace(options.Decision)
	options.Tag = strings.TrimSpace(options.Tag)
	options.Model = strings.TrimSpace(options.Model)
	for _, filter := range []struct {
		name  string
		value string
	}{
		{name: "decision", value: options.Decision},
		{name: "tag", value: options.Tag},
		{name: "model", value: options.Model},
	} {
		if utf8.RuneCountInString(filter.value) > maxSearchLength {
			return fmt.Errorf("%w: %s exceeds %d characters", ErrBadRequest, filter.name, maxSearchLength)
		}
	}
	options.Shape = strings.ToLower(strings.TrimSpace(options.Shape))
	if options.Shape != "" && options.Shape != "text" && options.Shape != "messages" && options.Shape != "tools" {
		return fmt.Errorf("%w: shape must be text, messages, or tools", ErrBadRequest)
	}
	return nil
}

func matchesFilters(probe ProbeDetail, options ListOptions) bool {
	if options.Decision != "" && probe.DecisionID != options.Decision {
		return false
	}
	if options.Tag != "" && !contains(probe.Tags, options.Tag) {
		return false
	}
	if options.Model != "" && probe.Model != options.Model {
		return false
	}
	if options.Shape != "" && !contains(probe.RequestShapes, options.Shape) {
		return false
	}
	if options.Query == "" {
		return true
	}
	needle := strings.ToLower(options.Query)
	haystack := strings.ToLower(strings.Join([]string{
		probe.ID,
		probe.DecisionID,
		probe.VariantID,
		probe.Model,
		probe.QueryPreview,
		probe.Notes,
		strings.Join(probe.Tags, " "),
	}, " "))
	return strings.Contains(haystack, needle)
}

func buildFacets(probes []ProbeDetail) Facets {
	facets := Facets{
		Decisions: map[string]int{},
		Tags:      map[string]int{},
		Models:    map[string]int{},
		Shapes:    map[string]int{},
	}
	for _, probe := range probes {
		facets.Decisions[probe.DecisionID]++
		if probe.Model != "" {
			facets.Models[probe.Model]++
		}
		for _, tag := range probe.Tags {
			facets.Tags[tag]++
		}
		for _, shape := range probe.RequestShapes {
			facets.Shapes[shape]++
		}
	}
	return facets
}

func materializeChatRequest(probe ProbeDetail) (ChatRequest, error) {
	messages := cloneObjects(probe.Messages)
	if len(messages) == 0 {
		text, err := materializeText(probe)
		if err != nil {
			return ChatRequest{}, err
		}
		messages = []map[string]any{{"role": "user", "content": text}}
	}
	request := ChatRequest{
		Model:    probe.Model,
		Messages: messages,
		Tools:    cloneObjects(probe.Tools),
	}
	data, err := json.Marshal(request)
	if err != nil {
		return ChatRequest{}, fmt.Errorf("%w: encode run plan: %w", ErrInvalid, err)
	}
	if len(data) > maxRequestBytes {
		return ChatRequest{}, fmt.Errorf("%w: materialized request exceeds %d byte limit", ErrBadRequest, maxRequestBytes)
	}
	return request, nil
}

func materializeEvalRequest(probe ProbeDetail) (EvalRequest, error) {
	request := EvalRequest{
		Model: probe.Model,
		Tools: cloneObjects(probe.Tools),
	}
	if len(probe.Messages) > 0 {
		request.Messages = cloneObjects(probe.Messages)
	} else {
		text, err := materializeText(probe)
		if err != nil {
			return EvalRequest{}, err
		}
		request.Text = text
	}
	data, err := json.Marshal(request)
	if err != nil {
		return EvalRequest{}, fmt.Errorf("%w: encode eval request: %w", ErrInvalid, err)
	}
	if len(data) > maxRequestBytes {
		return EvalRequest{}, fmt.Errorf("%w: materialized request exceeds %d byte limit", ErrBadRequest, maxRequestBytes)
	}
	return request, nil
}

func materializeText(probe ProbeDetail) (string, error) {
	if probe.Query == "" {
		return "", fmt.Errorf("%w: text probe has no query", ErrInvalid)
	}
	repeated, err := repeatLinesBounded(probe.Query, probe.Repeat, maxRequestBytes)
	if err != nil {
		return "", err
	}
	if probe.Padding == nil {
		return repeated, nil
	}
	extraSeparators := 1
	if probe.Padding.Placement == "around" && probe.Padding.Repeat > 1 {
		extraSeparators = 2
	}
	padding, err := repeatLinesBounded(probe.Padding.Text, probe.Padding.Repeat, maxRequestBytes-len(repeated)-extraSeparators)
	if err != nil {
		return "", err
	}
	var text string
	switch probe.Padding.Placement {
	case "before":
		text = joinNonEmpty(padding, repeated)
	case "after":
		text = joinNonEmpty(repeated, padding)
	case "around":
		midpoint := probe.Padding.Repeat / 2
		before := ""
		if midpoint > 0 {
			before, err = repeatLinesBounded(probe.Padding.Text, midpoint, maxRequestBytes)
			if err != nil {
				return "", err
			}
		}
		after, afterErr := repeatLinesBounded(probe.Padding.Text, probe.Padding.Repeat-midpoint, maxRequestBytes)
		if afterErr != nil {
			return "", afterErr
		}
		text = joinNonEmpty(before, repeated, after)
	default:
		return "", fmt.Errorf("%w: unsupported padding placement %q", ErrInvalid, probe.Padding.Placement)
	}
	if len(text) > maxRequestBytes {
		return "", fmt.Errorf("%w: materialized query exceeds %d byte limit", ErrBadRequest, maxRequestBytes)
	}
	return text, nil
}

func repeatLinesBounded(value string, repeat, limit int) (string, error) {
	if repeat < 1 || limit < 0 {
		return "", fmt.Errorf("%w: materialized query exceeds %d byte limit", ErrBadRequest, maxRequestBytes)
	}
	lineBytes := len(value)
	separators := repeat - 1
	if separators > limit || lineBytes > (limit-separators)/repeat {
		return "", fmt.Errorf("%w: materialized query exceeds %d byte limit", ErrBadRequest, maxRequestBytes)
	}
	total := lineBytes*repeat + separators
	if total > limit {
		return "", fmt.Errorf("%w: materialized query exceeds %d byte limit", ErrBadRequest, maxRequestBytes)
	}
	return strings.TrimSuffix(strings.Repeat(value+"\n", repeat), "\n"), nil
}

func joinNonEmpty(parts ...string) string {
	nonEmpty := parts[:0]
	for _, part := range parts {
		if part != "" {
			nonEmpty = append(nonEmpty, part)
		}
	}
	return strings.Join(nonEmpty, "\n")
}

func cloneObjects(items []map[string]any) []map[string]any {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]map[string]any, len(items))
	for i, item := range items {
		cloned[i] = make(map[string]any, len(item))
		for key, value := range item {
			cloned[i][key] = value
		}
	}
	return cloned
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func stableUnique(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func probeKey(decisionID, variantID string) string {
	return decisionID + "\x00" + variantID
}

func decodeStrictYAML(data []byte, target any) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
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

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
