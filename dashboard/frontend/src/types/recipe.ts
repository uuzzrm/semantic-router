export interface RecipeMetadataAuthor {
  name: string
  email?: string
  url?: string
}

export interface RecipeMetadata {
  schema_version: string
  id: string
  name: string
  version: string
  description: string
  authors?: Array<RecipeMetadataAuthor | string>
  license?: string
  tags?: string[]
  links?: Record<string, string>
}

export interface RecipeSourceFileHealth {
  present: boolean
  digest?: string
  size_bytes?: number
}

export interface RecipeSourceHealth {
  status: 'ready' | 'unmanaged' | 'invalid'
  files: Record<'metadata' | 'config' | 'probes' | 'dsl' | 'readme', RecipeSourceFileHealth>
  issues: string[]
}

export interface RecipeDigests {
  recipe?: string
  metadata?: string
  config?: string
  dsl?: string
  probes?: string
  readme?: string
}

export interface RecipeCounts {
  unified_models: number
  recipes: number
  decisions: number
  probes: number
}

export interface RecipeDescriptor {
  managed: boolean
  metadata?: RecipeMetadata
  readme?: string
  source_health: RecipeSourceHealth
  digests: RecipeDigests
  counts: RecipeCounts
}

export type RecipeProbeRequestShape = 'text' | 'messages' | 'tools'

export interface RecipeProbeExpectedRoute {
  decision: string
  recipe?: string
  algorithm?: string
  alias?: string
  plugins: string[]
  forbidden_plugins: string[]
  plugin_match?: string
  signals: Record<string, string[]>
  forbidden_signals: Record<string, string[]>
  signal_match?: string
}

export interface RecipeProbeSummary {
  id: string
  decision_id: string
  variant_id: string
  query_preview: string
  model?: string
  tags: string[]
  request_shapes: RecipeProbeRequestShape[]
  expected: RecipeProbeExpectedRoute
  editable: boolean
}

export interface RecipeProbeMessage extends Record<string, unknown> {
  role: string
  content: unknown
}

export interface RecipeProbeDetail extends RecipeProbeSummary {
  query?: string
  messages?: RecipeProbeMessage[]
  tools?: unknown[]
  repeat: number
  padding?: {
    text: string
    repeat: number
    placement: string
  }
  notes?: string
}

export interface RecipeProbeFacets {
  decisions: Record<string, number>
  tags: Record<string, number>
  models: Record<string, number>
  shapes: Record<string, number>
}

export interface RecipeProbePage {
  items: RecipeProbeSummary[]
  page: number
  page_size: number
  total: number
  total_pages: number
  facets: RecipeProbeFacets
  recipe_digest: string
}

export interface RecipeProbeListFilters {
  page?: number
  pageSize?: number
  query?: string
  decision?: string
  tag?: string
  model?: string
  requestShape?: string
}

export interface RecipeProbeValidationResult {
  probe_id: string
  recipe_digest: string
  passed: boolean
  expected: RecipeProbeExpectedRoute
  actual: {
    decision?: string
    model?: string
    recipe?: string
    algorithm?: string
    plugins: string[]
    recommended_models: string[]
    matched_signals: Record<string, string[]>
    trace_decisions: string[]
  }
  checks: Record<
    'decision' | 'model' | 'recipe' | 'algorithm' | 'plugins' | 'signals' | 'alias' | 'trace',
    boolean
  >
  failures: string[]
  latency_ms: number
  error?: string
}

export interface RecipeProbeRunPlan {
  probe_id: string
  recipe_digest: string
  model?: string
  messages: RecipeProbeMessage[]
  tools?: unknown[]
  request: Record<string, unknown>
  editable: boolean
}
