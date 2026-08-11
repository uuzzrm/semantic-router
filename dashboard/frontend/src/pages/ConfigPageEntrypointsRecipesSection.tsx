import { useRef, useState, type KeyboardEvent } from 'react'
import { useNavigate } from 'react-router-dom'

import type { RecipeProbeRunPlan } from '../types/recipe'
import { createProbePlaygroundInvocation } from '../types/playgroundInvocation'
import ConfigPageManagerLayout from './ConfigPageManagerLayout'
import ConfigPageMoMOverviewPanel from './ConfigPageMoMOverviewPanel'
import ConfigPageMoMProbesPanel from './ConfigPageMoMProbesPanel'
import ConfigPageMoMRoutingPanel from './ConfigPageMoMRoutingPanel'
import styles from './ConfigPageMoMWorkspace.module.css'
import type { ConfigData, NormalizedModel } from './configPageSupport'
import type { OpenEditModal, OpenViewModal } from './configPageRouterSectionSupport'

interface ConfigPageEntrypointsRecipesSectionProps {
  config: ConfigData
  isReadonly: boolean
  models: NormalizedModel[]
  saveConfig: (config: ConfigData) => Promise<void>
  openEditModal: OpenEditModal
  openViewModal: OpenViewModal
}

type MoMView = 'overview' | 'routing' | 'probes'

const VIEWS: Array<{ id: MoMView; label: string }> = [
  { id: 'overview', label: 'Overview' },
  { id: 'routing', label: 'Models & Routing' },
  { id: 'probes', label: 'Probes' },
]

export default function ConfigPageEntrypointsRecipesSection({
  config,
  isReadonly,
  models,
  saveConfig,
  openEditModal,
  openViewModal,
}: ConfigPageEntrypointsRecipesSectionProps) {
  const navigate = useNavigate()
  const [activeView, setActiveView] = useState<MoMView>('overview')
  const tabRefs = useRef<Array<HTMLButtonElement | null>>([])

  const handleTabKeyDown = (event: KeyboardEvent<HTMLButtonElement>, index: number) => {
    let nextIndex: number | null = null
    if (event.key === 'ArrowRight') nextIndex = (index + 1) % VIEWS.length
    if (event.key === 'ArrowLeft') nextIndex = (index - 1 + VIEWS.length) % VIEWS.length
    if (event.key === 'Home') nextIndex = 0
    if (event.key === 'End') nextIndex = VIEWS.length - 1
    if (nextIndex === null) return
    event.preventDefault()
    setActiveView(VIEWS[nextIndex].id)
    tabRefs.current[nextIndex]?.focus()
  }

  const launchProbe = (intent: 'run' | 'edit', plan: RecipeProbeRunPlan) => {
    navigate('/playground', {
      state: {
        playgroundInvocation: createProbePlaygroundInvocation(intent, plan),
      },
    })
  }

  return (
    <ConfigPageManagerLayout
      eyebrow="Dispatch"
      title="Mixture-of-Models"
      description="Operate unified models from one managed Recipe, reusable routing policies, and verified offline probes."
      configArea="Multi-recipe dispatch"
      scope="Active Recipe"
      panelEyebrow="Unified model workspace"
      panelTitle="One model surface, many model paths"
      panelDescription="Inspect the active Recipe, manage request-facing model IDs and routing, then exercise verified requests in Playground."
      pills={[
        { label: 'Models' },
        { label: 'Decisions' },
        { label: 'Mixture-of-Models', active: true },
      ]}
    >
      <div className={styles.tabs} role="tablist" aria-label="Mixture-of-Models views">
        {VIEWS.map((view, index) => (
          <button
            key={view.id}
            ref={(element) => {
              tabRefs.current[index] = element
            }}
            id={`mom-tab-${view.id}`}
            type="button"
            role="tab"
            aria-selected={activeView === view.id}
            aria-controls="mom-active-panel"
            tabIndex={activeView === view.id ? 0 : -1}
            className={`${styles.tab} ${activeView === view.id ? styles.activeTab : ''}`}
            onClick={() => setActiveView(view.id)}
            onKeyDown={(event) => handleTabKeyDown(event, index)}
          >
            {view.label}
          </button>
        ))}
      </div>

      <div
        id="mom-active-panel"
        className={styles.tabPanel}
        role="tabpanel"
        aria-labelledby={`mom-tab-${activeView}`}
      >
        {activeView === 'overview' ? <ConfigPageMoMOverviewPanel /> : null}
        {activeView === 'routing' ? (
          <ConfigPageMoMRoutingPanel
            config={config}
            isReadonly={isReadonly}
            models={models}
            saveConfig={saveConfig}
            openEditModal={openEditModal}
            openViewModal={openViewModal}
          />
        ) : null}
        {activeView === 'probes' ? <ConfigPageMoMProbesPanel onLaunch={launchProbe} /> : null}
      </div>
    </ConfigPageManagerLayout>
  )
}
