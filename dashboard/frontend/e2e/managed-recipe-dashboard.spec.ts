import { expect, test, type Page } from '@playwright/test'

import { mockAuthenticatedAppShell } from './support/auth'

const config = {
  version: 'v0.3',
  providers: { defaults: { default_model: 'leaf-a' }, models: [{ name: 'leaf-a' }] },
  routing: { modelCards: [{ name: 'leaf-a' }], decisions: [] },
  entrypoints: [{ model_names: ['vllm-sr/mom-balanced-v1'], recipe: 'balanced' }],
  recipes: [
    {
      name: 'balanced',
      description: 'Balanced objective',
      routing: {
        decisions: [
          {
            name: 'decision-a',
            priority: 100,
            rules: { operator: 'AND', conditions: [] },
            modelRefs: [{ model: 'leaf-a', use_reasoning: false }],
          },
        ],
      },
    },
  ],
}

const expected = {
  decision: 'decision-a',
  recipe: 'balanced',
  algorithm: 'static',
  alias: 'leaf-a',
  plugins: [],
  forbidden_plugins: [],
  plugin_match: 'contains',
  signals: {},
  forbidden_signals: {},
  signal_match: 'contains',
}

const summary = {
  id: 'decision-a:variant-a',
  decision_id: 'decision-a',
  variant_id: 'variant-a',
  query_preview: 'Please correct the earlier answer.',
  model: 'vllm-sr/mom-balanced-v1',
  tags: ['messages', 'verified'],
  request_shapes: ['messages', 'tools'],
  expected,
  editable: true,
}

const probeMessages = [
  { role: 'user', content: 'Give the first answer.' },
  { role: 'assistant', content: 'This is the first answer.' },
  { role: 'user', content: 'Please correct the earlier answer.' },
]

const probeTools = [
  {
    type: 'function',
    function: {
      name: 'lookup_source',
      description: 'Look up a source.',
      parameters: { type: 'object', properties: {}, required: [] },
    },
  },
]

async function mockManagedRecipeWorkspace(page: Page) {
  await mockAuthenticatedAppShell(page)
  await page.route('**/api/router/config/all', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(config),
    })
  })
  await page.route('**/api/router/config/global', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: '{}' })
  })
  await page.route('**/api/status', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ overall: 'healthy', services: [], models: { models: [] } }),
    })
  })
  await page.route('**/api/recipe', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        managed: true,
        metadata: {
          schema_version: 'vllm-sr/recipe-metadata/v1',
          id: 'multi-objective',
          name: 'Multi-Objective Mixture-of-Models',
          version: '0.1.0',
          description: 'One unified model surface over isolated routing objectives.',
          authors: [{ name: 'vLLM Semantic Router Contributors' }],
          license: 'Apache-2.0',
          tags: ['mixture-of-models'],
          links: { source: 'https://example.com/recipe' },
        },
        readme: '# Managed Recipe\n\nInspectable and testable.',
        source_health: {
          status: 'ready',
          files: Object.fromEntries(
            ['metadata', 'config', 'probes', 'dsl', 'readme'].map((name) => [
              name,
              { present: true, digest: `sha256:${name}` },
            ]),
          ),
          issues: [],
        },
        digests: { recipe: 'sha256:recipe' },
        counts: { unified_models: 1, recipes: 1, decisions: 1, probes: 51 },
      }),
    })
  })
  await page.route('**/api/router/v1/models*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        object: 'list',
        data: [
          {
            id: 'vllm-sr/mom-balanced-v1',
            owned_by: 'vllm-semantic-router',
            routing: { resolution: 'virtual', selectable: true, recipe: 'balanced' },
          },
        ],
      }),
    })
  })
}

async function mockProbeAPI(page: Page, requestLog: string[]) {
  await page.route('**/api/recipe/probes**', async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    requestLog.push(`${request.method()} ${url.pathname}${url.search}`)
    const path = url.pathname

    if (path.endsWith('/validate')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          probe_id: summary.id,
          recipe_digest: 'sha256:recipe',
          passed: true,
          expected,
          actual: {
            decision: 'decision-a',
            model: 'vllm-sr/mom-balanced-v1',
            recipe: 'balanced',
            algorithm: 'static',
            plugins: [],
            recommended_models: ['leaf-a'],
            matched_signals: {},
            trace_decisions: ['decision-a'],
          },
          checks: {
            decision: true,
            model: true,
            recipe: true,
            algorithm: true,
            plugins: true,
            signals: true,
            alias: true,
            trace: true,
          },
          failures: [],
          latency_ms: 8,
        }),
      })
      return
    }

    if (path.endsWith('/run-plan')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          probe_id: summary.id,
          recipe_digest: 'sha256:recipe',
          model: summary.model,
          messages: probeMessages,
          tools: probeTools,
          request: {
            model: summary.model,
            messages: probeMessages,
            tools: probeTools,
            temperature: 0,
          },
          editable: true,
        }),
      })
      return
    }

    if (path !== '/api/recipe/probes') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          ...summary,
          messages: probeMessages,
          tools: probeTools,
          repeat: 1,
          notes: 'A verified multi-turn correction fixture.',
        }),
      })
      return
    }

    const pageNumber = Number(url.searchParams.get('page') ?? '1')
    const item =
      pageNumber === 1
        ? summary
        : {
            ...summary,
            id: 'decision-a:variant-b',
            variant_id: 'variant-b',
            query_preview: 'Second page probe.',
          }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        items: [item],
        page: pageNumber,
        page_size: 50,
        total: 51,
        total_pages: 2,
        facets: {
          decisions: { 'decision-a': 51 },
          tags: { messages: 1, verified: 51 },
          models: { 'vllm-sr/mom-balanced-v1': 51 },
          shapes: { messages: 1, text: 50, tools: 1 },
        },
        recipe_digest: 'sha256:recipe',
      }),
    })
  })
}

test.beforeEach(async ({ page }) => {
  await mockManagedRecipeWorkspace(page)
})

test('shows managed metadata and pages, inspects, and validates probes server-side', async ({
  page,
}) => {
  const requests: string[] = []
  let chatRequests = 0
  await mockProbeAPI(page, requests)
  await page.route('**/api/router/v1/chat/completions', async (route) => {
    chatRequests += 1
    await route.abort()
  })

  await page.goto('/config/entrypoints-recipes')
  await expect(
    page.getByRole('heading', { name: 'Multi-Objective Mixture-of-Models' }),
  ).toBeVisible()
  await expect(page.getByText('51', { exact: true })).toBeVisible()
  await expect(page.getByText('metadata.yaml')).toBeVisible()

  await page.getByRole('tab', { name: 'Probes' }).click()
  await expect(page.getByText('Please correct the earlier answer.')).toBeVisible()
  expect(requests).toContain('GET /api/recipe/probes?page=1&page_size=50')

  await page.getByRole('button', { name: `View ${summary.id} details` }).click()
  await expect(page.getByText('A verified multi-turn correction fixture.')).toBeVisible()
  await expect(page.getByText('lookup_source')).toBeVisible()

  await page.getByRole('button', { name: 'Validate' }).click()
  await expect(page.getByText('Route validated')).toBeVisible()
  expect(requests).toContain('POST /api/recipe/probes/decision-a/variant-a/validate')
  expect(chatRequests).toBe(0)

  await page.getByRole('button', { name: 'Next' }).click()
  await expect(page.getByText('Second page probe.')).toBeVisible()
  expect(requests).toContain('GET /api/recipe/probes?page=2&page_size=50')
})

test('Edit opens a clean composer without sending and preserves the exact structured request', async ({
  page,
}) => {
  const requests: string[] = []
  const chatBodies: Array<Record<string, unknown>> = []
  await mockProbeAPI(page, requests)
  await page.route('**/api/router/v1/chat/completions', async (route) => {
    chatBodies.push(route.request().postDataJSON() as Record<string, unknown>)
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        choices: [{ index: 0, message: { content: 'Edited probe answer.' } }],
      }),
    })
  })
  await page.addInitScript(() => {
    window.localStorage.setItem(
      'sr:chat:conversations',
      JSON.stringify([
        {
          id: 'legacy-conversation',
          createdAt: 1,
          updatedAt: 1,
          payload: [
            { id: 'legacy-message', role: 'user', content: 'Legacy conversation', timestamp: 1 },
          ],
        },
      ]),
    )
  })

  await page.goto('/config/entrypoints-recipes')
  await page.getByRole('tab', { name: 'Probes' }).click()
  await page.getByRole('button', { name: 'Edit' }).click()

  await expect(page).toHaveURL(/\/playground$/)
  const composer = page.getByPlaceholder('Ask me anything...')
  await expect(composer).toHaveValue('Please correct the earlier answer.')
  const transcript = page.getByTestId('chat-transcript')
  await expect(transcript.getByText('This is the first answer.')).toBeVisible()
  await expect(transcript.getByText('Legacy conversation')).toHaveCount(0)
  expect(chatBodies).toHaveLength(0)

  await composer.fill('Correct it and cite two sources.')
  await page.getByRole('button', { name: 'Send message' }).click()
  await expect(page.getByText('Edited probe answer.')).toBeVisible()
  expect(chatBodies).toEqual([
    {
      model: 'vllm-sr/mom-balanced-v1',
      messages: [
        probeMessages[0],
        probeMessages[1],
        { role: 'user', content: 'Correct it and cite two sources.' },
      ],
      tools: probeTools,
      temperature: 0,
      stream: true,
    },
  ])
})

test('Run opens a clean chat and immediately sends the materialized probe request', async ({
  page,
}) => {
  const requests: string[] = []
  const chatBodies: Array<Record<string, unknown>> = []
  await mockProbeAPI(page, requests)
  await page.route('**/api/router/v1/chat/completions', async (route) => {
    chatBodies.push(route.request().postDataJSON() as Record<string, unknown>)
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ choices: [{ index: 0, message: { content: 'Probe answer.' } }] }),
    })
  })

  await page.goto('/config/entrypoints-recipes')
  await page.getByRole('tab', { name: 'Probes' }).click()
  await page.getByRole('button', { name: 'Run' }).click()

  await expect(page).toHaveURL(/\/playground$/)
  await expect(page.getByText('Probe answer.')).toBeVisible()
  expect(requests).toContain('POST /api/recipe/probes/decision-a/variant-a/run-plan')
  expect(chatBodies).toEqual([
    {
      model: 'vllm-sr/mom-balanced-v1',
      messages: probeMessages,
      tools: probeTools,
      temperature: 0,
      stream: true,
    },
  ])
})
