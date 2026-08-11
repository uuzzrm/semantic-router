import type {
  RecipeDescriptor,
  RecipeProbeDetail,
  RecipeProbeListFilters,
  RecipeProbePage,
  RecipeProbeRunPlan,
  RecipeProbeValidationResult,
} from '../types/recipe'

async function readJson<T>(response: Response): Promise<T> {
  if (response.ok) return (await response.json()) as T

  const fallback = `${response.status} ${response.statusText}`.trim()
  try {
    const body = (await response.json()) as { error?: string; message?: string }
    throw new Error(body.error || body.message || fallback)
  } catch (error) {
    if (error instanceof SyntaxError) throw new Error(fallback)
    throw error
  }
}

export function buildRecipeProbeQuery(filters: RecipeProbeListFilters): string {
  const query = new URLSearchParams()
  query.set('page', String(filters.page ?? 1))
  query.set('page_size', String(filters.pageSize ?? 50))
  if (filters.query?.trim()) query.set('q', filters.query.trim())
  if (filters.decision) query.set('decision', filters.decision)
  if (filters.tag) query.set('tag', filters.tag)
  if (filters.model) query.set('model', filters.model)
  if (filters.requestShape) query.set('shape', filters.requestShape)
  return query.toString()
}

export async function getActiveRecipe(signal?: AbortSignal): Promise<RecipeDescriptor> {
  const response = await fetch('/api/recipe', { signal })
  const payload = (await response.json()) as RecipeDescriptor | { error?: string; message?: string }
  if ('managed' in payload && typeof payload.managed === 'boolean') return payload
  const message = 'error' in payload ? payload.error : 'message' in payload ? payload.message : ''
  throw new Error(message || `${response.status} ${response.statusText}`.trim())
}

export async function listRecipeProbes(
  filters: RecipeProbeListFilters,
  signal?: AbortSignal,
): Promise<RecipeProbePage> {
  const query = buildRecipeProbeQuery(filters)
  return readJson<RecipeProbePage>(await fetch(`/api/recipe/probes?${query}`, { signal }))
}

function probePath(decision: string, variant: string): string {
  return `/api/recipe/probes/${encodeURIComponent(decision)}/${encodeURIComponent(variant)}`
}

export async function getRecipeProbe(
  decision: string,
  variant: string,
  signal?: AbortSignal,
): Promise<RecipeProbeDetail> {
  return readJson<RecipeProbeDetail>(await fetch(probePath(decision, variant), { signal }))
}

export async function validateRecipeProbe(
  decision: string,
  variant: string,
  recipeDigest: string,
  signal?: AbortSignal,
): Promise<RecipeProbeValidationResult> {
  const response = await fetch(`${probePath(decision, variant)}/validate`, {
    method: 'POST',
    headers: { 'If-Match': `"${recipeDigest}"` },
    signal,
  })
  const payload = (await response.json()) as
    | RecipeProbeValidationResult
    | { error?: string; message?: string }
  if ('passed' in payload && typeof payload.passed === 'boolean') return payload
  const message = 'error' in payload ? payload.error : 'message' in payload ? payload.message : ''
  throw new Error(message || `${response.status} ${response.statusText}`.trim())
}

export async function createRecipeProbeRunPlan(
  decision: string,
  variant: string,
  recipeDigest: string,
  signal?: AbortSignal,
): Promise<RecipeProbeRunPlan> {
  return readJson<RecipeProbeRunPlan>(
    await fetch(`${probePath(decision, variant)}/run-plan`, {
      method: 'POST',
      headers: { 'If-Match': `"${recipeDigest}"` },
      signal,
    }),
  )
}
