import { fireEvent, screen, waitFor } from '@testing-library/react'
import { act } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { QASession } from '@/lib/types'
import { useChatStore } from '@/stores/chat-store'
import { renderWithProviders } from '@/test/render'

import { ChatPage } from './page'

function jsonResponse(body: unknown, init?: ResponseInit) {
  return new Response(JSON.stringify(body), {
    headers: { 'Content-Type': 'application/json', ...init?.headers },
    status: init?.status ?? 200,
    statusText: init?.statusText,
  })
}

function pageResponse(data: unknown[]) {
  return jsonResponse({
    data,
    page: { page: 1, pageSize: 20, total: data.length },
    requestId: 'req-page',
  })
}

function createSession(): QASession {
  return {
    createdAt: '2026-06-30T00:00:00Z',
    id: 'session-1',
    messageCount: 0,
    status: 'active',
    title: 'QA session',
    updatedAt: '2026-06-30T00:00:00Z',
  }
}

function getLastAssistantMessage() {
  const messages = useChatStore.getState().messagesBySession['session-1'] ?? []
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index]
    if (message?.role === 'assistant') return message
  }
  throw new Error('expected an assistant message')
}

describe('ChatPage stream sequencing', () => {
  beforeEach(() => {
    useChatStore.setState({
      activeId: 'session-1',
      error: null,
      lastFailedMsg: null,
      messagesBySession: { 'session-1': [] },
      sessionIds: ['session-1'],
      sessions: [createSession()],
      streaming: false,
    })
  })

  it('ignores out-of-order answer.completed events without ending the active stream', async () => {
    const encoder = new TextEncoder()
    let streamController: ReadableStreamDefaultController<Uint8Array> | undefined
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const request = input instanceof Request ? input : new Request(input, init)
      const url = new URL(request.url)

      if (request.method === 'GET' && url.pathname.endsWith('/qa-sessions')) {
        return pageResponse([createSession()])
      }
      if (request.method === 'GET' && url.pathname.endsWith('/qa-sessions/session-1/messages')) {
        return pageResponse([])
      }
      if (request.method === 'POST' && url.pathname.endsWith('/qa-sessions/session-1/messages')) {
        const stream = new ReadableStream<Uint8Array>({
          start(controller) {
            streamController = controller
          },
        })
        return new Response(stream, {
          headers: { 'Content-Type': 'text/event-stream' },
          status: 200,
        })
      }

      return jsonResponse({ data: {}, requestId: 'req-default' })
    })
    vi.stubGlobal('fetch', fetchMock)

    renderWithProviders(<ChatPage />)

    const input = screen.getByRole('textbox')
    fireEvent.change(input, { target: { value: 'question' } })
    fireEvent.keyDown(input, { code: 'Enter', key: 'Enter' })

    await waitFor(() => expect(streamController).toBeDefined())
    await waitFor(() => expect(input).toBeDisabled())

    const emit = async (event: string, data: Record<string, unknown>, id?: number) => {
      await act(async () => {
        const idLine = id === undefined ? '' : `id: ${id}\n`
        streamController?.enqueue(
          encoder.encode(`event: ${event}\n${idLine}data: ${JSON.stringify(data)}\n\n`),
        )
        await new Promise((resolve) => window.setTimeout(resolve, 0))
      })
    }

    await emit('answer.delta', { content: 'first ' }, 2)
    await waitFor(() => expect(getLastAssistantMessage().content).toBe('first '))

    await emit('answer.completed', { responseRunId: 'run-1' }, 1)
    await act(async () => {
      await new Promise((resolve) => window.setTimeout(resolve, 0))
    })

    expect(input).toBeDisabled()
    expect(useChatStore.getState().streaming).toBe(true)
    expect(getLastAssistantMessage()).toMatchObject({
      content: 'first ',
      status: 'streaming',
    })

    await emit('answer.delta', { content: 'second' }, 3)
    await waitFor(() => expect(getLastAssistantMessage().content).toBe('first second'))

    await emit('answer.completed', { responseRunId: 'run-1' }, 4)
    await waitFor(() => expect(useChatStore.getState().streaming).toBe(false))

    expect(input).not.toBeDisabled()
    expect(getLastAssistantMessage()).toMatchObject({
      content: 'first second',
      status: 'completed',
    })
  })
})
