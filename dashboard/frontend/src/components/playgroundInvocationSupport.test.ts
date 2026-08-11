import { describe, expect, it } from 'vitest'

import {
  createProbePlaygroundTask,
  materializeEditedProbeRequest,
  preparePlaygroundInvocation,
  toPlaygroundHistory,
} from './playgroundInvocationSupport'
import type { PlaygroundInvocation } from '../types/playgroundInvocation'

const invocation = (intent: 'run' | 'edit'): PlaygroundInvocation => ({
  version: 1,
  intent,
  source: 'recipe-probe',
  probeId: 'balanced_recovery/semantic_reask',
  recipeDigest: 'sha256:recipe',
  editable: true,
  model: 'vllm-sr/mom-balanced-v1',
  messages: [
    { role: 'user', content: 'Explain vector clocks.' },
    { role: 'assistant', content: 'A vector clock is a logical timestamp.' },
    { role: 'user', content: 'Explain vector clocks.' },
  ],
  tools: [{ type: 'function', function: { name: 'lookup', parameters: {} } }],
  request: { temperature: 0 },
})

describe('playgroundInvocationSupport', () => {
  it('prepares the final user turn as the editable prompt', () => {
    const prepared = preparePlaygroundInvocation(invocation('edit'))

    expect(prepared.prompt).toBe('Explain vector clocks.')
    expect(prepared.history).toHaveLength(2)
    expect(prepared.exactRequest).toMatchObject({
      model: 'vllm-sr/mom-balanced-v1',
      temperature: 0,
      tools: expect.any(Array),
      messages: expect.any(Array),
    })
  })

  it('replaces only the editable turn and permits a selected model change', () => {
    const prepared = preparePlaygroundInvocation(invocation('edit'))

    const request = materializeEditedProbeRequest(
      prepared,
      'Explain them with two concurrent writers.',
      'vllm-sr/mom-flash-v1',
    )

    expect(request.model).toBe('vllm-sr/mom-flash-v1')
    expect(request.messages).toEqual([
      { role: 'user', content: 'Explain vector clocks.' },
      { role: 'assistant', content: 'A vector clock is a logical timestamp.' },
      { role: 'user', content: 'Explain them with two concurrent writers.' },
    ])
  })

  it('creates display history without changing the exact request', () => {
    const prepared = preparePlaygroundInvocation(invocation('run'))
    let nextId = 0

    const history = toPlaygroundHistory(prepared.history, () => `message-${nextId++}`)

    expect(history.map((message) => message.role)).toEqual(['user', 'assistant'])
    expect(history.map((message) => message.content)).toEqual([
      'Explain vector clocks.',
      'A vector clock is a logical timestamp.',
    ])
  })

  it('builds an isolated task that sends the probe tools without executing them', () => {
    const task = createProbePlaygroundTask(
      preparePlaygroundInvocation(invocation('run')),
      'conversation-1',
      'vllm-sr/mom-balanced-v1',
    )

    expect(task).toMatchObject({
      conversationId: 'conversation-1',
      prompt: 'Explain vector clocks.',
      requestOptions: {
        enableClawMode: false,
        enableWebSearch: false,
        executeToolCalls: false,
        model: 'vllm-sr/mom-balanced-v1',
      },
      exactRequest: {
        model: 'vllm-sr/mom-balanced-v1',
        temperature: 0,
        tools: expect.any(Array),
        messages: expect.any(Array),
      },
      appendPromptMessage: true,
    })
  })

  it('preserves a terminal tool result as history without inventing a user turn', () => {
    const structured: PlaygroundInvocation = {
      ...invocation('run'),
      editable: false,
      messages: [
        { role: 'user', content: 'Look up the policy.' },
        {
          role: 'assistant',
          content: '',
          tool_calls: [
            {
              id: 'call-1',
              type: 'function',
              function: { name: 'lookup', arguments: '{}' },
            },
          ],
        },
        { role: 'tool', tool_call_id: 'call-1', content: 'Policy result.' },
      ],
    }

    const prepared = preparePlaygroundInvocation(structured)
    const history = toPlaygroundHistory(prepared.history, () => crypto.randomUUID())
    const task = createProbePlaygroundTask(prepared, 'conversation-2', structured.model!)

    expect(prepared.editable).toBe(false)
    expect(prepared.prompt).toBe('')
    expect(history).toHaveLength(2)
    expect(history[1]).toMatchObject({
      role: 'assistant',
      toolCalls: [{ id: 'call-1', status: 'completed' }],
      toolResults: [{ callId: 'call-1', content: 'Policy result.' }],
    })
    expect(task.appendPromptMessage).toBe(false)
  })
})
