import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const readSource = (name: string) => readFileSync(new URL(name, import.meta.url), 'utf8')

describe('Mixture-of-Models workspace contracts', () => {
  it('keeps the existing route section as a narrow accessible three-view orchestrator', () => {
    const source = readSource('./ConfigPageEntrypointsRecipesSection.tsx')

    expect(source).toContain("type MoMView = 'overview' | 'routing' | 'probes'")
    expect(source).toContain('role="tablist"')
    expect(source).toContain('role="tab"')
    expect(source).toContain('role="tabpanel"')
    expect(source).toContain('<ConfigPageMoMRoutingPanel')
    expect(source).toContain('<ConfigPageMoMOverviewPanel')
    expect(source).toContain('<ConfigPageMoMProbesPanel')
    expect(source).not.toContain('AMD models')
  })

  it('uses server paging, lazy detail, strict validation, and fresh run plans', () => {
    const source = readSource('./ConfigPageMoMProbesPanel.tsx')

    expect(source).toContain('RECIPE_PROBE_PAGE_SIZE')
    expect(source).toContain('listRecipeProbes(')
    expect(source).toContain('getRecipeProbe(')
    expect(source).toContain('validateRecipeProbe(')
    expect(source).toContain('createRecipeProbeRunPlan(')
    expect(source).not.toContain('/api/v1/eval')
    expect(source).not.toContain('simulate')
  })

  it('does not load package-controlled images from Recipe documentation', () => {
    const source = readSource('./ConfigPageMoMOverviewPanel.tsx')

    expect(source).toContain('<MarkdownRenderer content={recipe.readme} allowImages={false} />')
  })
})
