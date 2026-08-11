import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  buildRecipeProbeQuery,
  createRecipeProbeRunPlan,
  getActiveRecipe,
  getRecipeProbe,
  listRecipeProbes,
  validateRecipeProbe,
} from './recipeApi'

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('Recipe API client', () => {
  it('preserves invalid managed Recipe health returned with a 422', async () => {
    const descriptor = {
      managed: true,
      source_health: { status: 'invalid', files: {}, issues: ['probes.yaml is invalid'] },
      digests: {},
      counts: { unified_models: 0, recipes: 0, decisions: 0, probes: 0 },
    }
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => new Response(JSON.stringify(descriptor), { status: 422 })),
    )

    await expect(getActiveRecipe()).resolves.toEqual(descriptor)
  })

  it('builds bounded server-side probe pagination and filters', () => {
    const query = new URLSearchParams(
      buildRecipeProbeQuery({
        page: 3,
        pageSize: 25,
        query: '  safety review  ',
        decision: 'private/route',
        tag: 'language:zh',
        model: 'vllm-sr/mom private',
        requestShape: 'tools',
      }),
    )

    expect(Object.fromEntries(query)).toEqual({
      page: '3',
      page_size: '25',
      q: 'safety review',
      decision: 'private/route',
      tag: 'language:zh',
      model: 'vllm-sr/mom private',
      shape: 'tools',
    })
  })

  it('forwards list abort signals and never requests an unpaged collection', async () => {
    const controller = new AbortController()
    const fetchMock = vi.fn(
      async () => new Response(JSON.stringify({ items: [] }), { status: 200 }),
    )
    vi.stubGlobal('fetch', fetchMock)

    await listRecipeProbes({ page: 2, pageSize: 50, decision: 'secure' }, controller.signal)

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/recipe/probes?page=2&page_size=50&decision=secure',
      { signal: controller.signal },
    )
  })

  it('encodes both stable probe path segments', async () => {
    const fetchMock = vi.fn(async () => new Response('{}', { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)

    await getRecipeProbe('private/route', 'zh review')

    expect(fetchMock).toHaveBeenCalledWith('/api/recipe/probes/private%2Froute/zh%20review', {
      signal: undefined,
    })
  })

  it('preserves a strict failed validation returned with an upstream 502', async () => {
    const failedValidation = {
      probe_id: 'secure/failure',
      recipe_digest: 'sha256:recipe',
      passed: false,
      expected: { decision: 'secure' },
      actual: {
        decision: 'fallback',
        plugins: [],
        recommended_models: [],
        matched_signals: {},
        trace_decisions: [],
      },
      checks: {},
      failures: ['decision: expected secure, got fallback'],
      latency_ms: 11,
      error: 'router eval unavailable',
    }
    const fetchMock = vi.fn(
      async () => new Response(JSON.stringify(failedValidation), { status: 502 }),
    )
    vi.stubGlobal('fetch', fetchMock)

    await expect(validateRecipeProbe('secure', 'failure', 'sha256:recipe')).resolves.toEqual(
      failedValidation,
    )
    expect(fetchMock).toHaveBeenCalledWith('/api/recipe/probes/secure/failure/validate', {
      method: 'POST',
      headers: { 'If-Match': '"sha256:recipe"' },
      signal: undefined,
    })
  })

  it('fetches a fresh run plan for each launch action', async () => {
    const fetchMock = vi.fn(
      async () =>
        new Response(
          JSON.stringify({ probe_id: 'secure/run', messages: [], request: {}, editable: true }),
          {
            status: 200,
          },
        ),
    )
    vi.stubGlobal('fetch', fetchMock)

    await createRecipeProbeRunPlan('secure', 'run', 'sha256:recipe')
    await createRecipeProbeRunPlan('secure', 'run', 'sha256:recipe')

    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(fetchMock).toHaveBeenLastCalledWith('/api/recipe/probes/secure/run/run-plan', {
      method: 'POST',
      headers: { 'If-Match': '"sha256:recipe"' },
      signal: undefined,
    })
  })
})
