import { afterEach, describe, expect, it, vi } from 'vitest'

import { runPlaygroundTask } from './chatTaskExecution'
import type { Message, PlaygroundTask } from './ChatComponentTypes'
import type { ToolDefinition } from '../tools'

const probeTool: ToolDefinition = {
  type: 'function',
  function: {
    name: 'lookup_policy',
    description: 'Look up a policy.',
    parameters: { type: 'object', properties: {}, required: [] },
  },
}

describe('runPlaygroundTask exact requests', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('sends the server materialized probe request without injecting or executing tools', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          choices: [
            {
              index: 0,
              message: {
                content: null,
                tool_calls: [
                  {
                    id: 'call-1',
                    type: 'function',
                    function: { name: 'lookup_policy', arguments: '{}' },
                  },
                ],
              },
            },
          ],
        }),
        { headers: { 'content-type': 'application/json' } },
      ),
    )
    vi.stubGlobal('fetch', fetchMock)

    const task: PlaygroundTask = {
      id: 'task-1',
      conversationId: 'conversation-1',
      prompt: 'Check the policy.',
      createdAt: 1,
      requestOptions: {
        enableClawMode: false,
        enableWebSearch: false,
        executeToolCalls: false,
        model: 'vllm-sr/mom-balanced-v1',
      },
      exactRequest: {
        model: 'vllm-sr/mom-balanced-v1',
        messages: [{ role: 'user', content: 'Check the policy.' }],
        tools: [probeTool],
        temperature: 0,
      },
    }
    const buildTaskTools = vi.fn(() => [probeTool])
    const executeTools = vi.fn(async () => [])
    let messages: Message[] = []
    let nextId = 0

    await runPlaygroundTask({
      buildTaskTools,
      clawManagementDisabled: false,
      clearConversationActiveTask: vi.fn(),
      endpoint: '/api/router/v1/chat/completions',
      executeTools,
      expandedToolCardCount: 0,
      generateId: () => `message-${nextId++}`,
      getConversationMessagesSnapshot: () => messages,
      getCurrentConversationId: () => 'conversation-1',
      registerAbortController: vi.fn(),
      setConversationError: vi.fn(),
      setConversationHeaderReveal: vi.fn(),
      setConversationThinking: vi.fn(),
      setExpandedToolCards: vi.fn(),
      task,
      updateConversationMessages: (_conversationId, updater) => {
        messages = updater(messages)
      },
    })

    expect(buildTaskTools).not.toHaveBeenCalled()
    expect(executeTools).not.toHaveBeenCalled()
    expect(fetchMock).toHaveBeenCalledOnce()
    const [, requestInit] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(JSON.parse(String(requestInit.body))).toEqual({
      model: 'vllm-sr/mom-balanced-v1',
      messages: [{ role: 'user', content: 'Check the policy.' }],
      tools: [probeTool],
      temperature: 0,
      stream: true,
    })
    expect(messages[messages.length - 1]).toMatchObject({
      role: 'assistant',
      isStreaming: false,
      toolCalls: [{ status: 'skipped' }],
    })
  })
})
