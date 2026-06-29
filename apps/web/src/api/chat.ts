/**
 * Chat SSE streaming + Knowledge query — Gateway OpenAPI paths.
 *
 * SSE endpoint:  POST /qa-sessions/{sessionId}/messages (Accept: text/event-stream)
 * Knowledge:     POST /knowledge-queries
 *
 * Event types per OpenAPI QASseEventType:
 *   message.created  agent.iteration.started  reasoning.step
 *   tool.started     tool.completed           tool.failed
 *   answer.delta     citation.delta           answer.completed
 *   error            heartbeat
 */

import type {
  KnowledgeQueryRequest,
  KnowledgeQuerySummary,
  QAMessage,
  QASseEvent,
  QASseEventType,
} from '@/lib/types'

import { apiClient, buildQuery, gatewayRequest } from './client'

// ---------------------------------------------------------------------------
// SSE handlers interface
// ---------------------------------------------------------------------------

export interface ChatStreamHandlers {
  onMessageCreated?: (data: Record<string, unknown> & { seq: number }) => void
  onAgentIterationStarted?: (data: Record<string, unknown> & { seq: number }) => void
  onReasoningStep?: (data: Record<string, unknown> & { seq: number }) => void
  onToolStarted?: (data: Record<string, unknown> & { seq: number }) => void
  onToolCompleted?: (data: Record<string, unknown> & { seq: number }) => void
  onToolFailed?: (data: Record<string, unknown> & { seq: number }) => void
  onAnswerDelta?: (data: Record<string, unknown> & { seq: number }) => void
  onCitationDelta?: (data: Record<string, unknown> & { seq: number }) => void
  onAnswerCompleted?: (data: Record<string, unknown> & { seq: number }) => void
  onError?: (data: { code?: string; message: string; fatal?: boolean; seq: number }) => void
  onAbort?: () => void
}

function dispatch(event: QASseEventType, data: unknown, handlers: ChatStreamHandlers): void {
  switch (event) {
    case 'message.created':
      handlers.onMessageCreated?.(data as Record<string, unknown> & { seq: number })
      break
    case 'agent.iteration.started':
      handlers.onAgentIterationStarted?.(data as Record<string, unknown> & { seq: number })
      break
    case 'reasoning.step':
      handlers.onReasoningStep?.(data as Record<string, unknown> & { seq: number })
      break
    case 'tool.started':
      handlers.onToolStarted?.(data as Record<string, unknown> & { seq: number })
      break
    case 'tool.completed':
      handlers.onToolCompleted?.(data as Record<string, unknown> & { seq: number })
      break
    case 'tool.failed':
      handlers.onToolFailed?.(data as Record<string, unknown> & { seq: number })
      break
    case 'answer.delta':
      handlers.onAnswerDelta?.(data as Record<string, unknown> & { seq: number })
      break
    case 'citation.delta':
      handlers.onCitationDelta?.(data as Record<string, unknown> & { seq: number })
      break
    case 'answer.completed':
      handlers.onAnswerCompleted?.(data as Record<string, unknown> & { seq: number })
      break
    case 'error':
      handlers.onError?.(data as { code?: string; message: string; fatal?: boolean; seq: number })
      break
    default:
      break
  }
}

/** Build auth + request-id headers for SSE requests. */
function buildStreamHeaders(): HeadersInit {
  const token = apiClient.getToken()
  const headers: Record<string, string> = {
    'X-Request-Id': `req-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`,
    'Content-Type': 'application/json',
    Accept: 'text/event-stream',
  }
  if (token) {
    headers.Authorization = `Bearer ${token}`
  }
  return headers
}

/** Combine two AbortSignals into one merged signal. */
function mergeAbortSignals(a: AbortSignal, b: AbortSignal): AbortSignal {
  if (a.aborted) return a
  if (b.aborted) return b
  const controller = new AbortController()
  const handler = () => controller.abort()
  a.addEventListener('abort', handler, { once: true })
  b.addEventListener('abort', handler, { once: true })
  return controller.signal
}

// ---------------------------------------------------------------------------
// POST /qa-sessions/{sessionId}/messages  (SSE stream)
// ---------------------------------------------------------------------------

/**
 * Initiate a streaming QA chat request via SSE.
 *
 * @param sessionId  QA session id (path parameter)
 * @param message    User message text (required body field)
 * @param handlers   Event-type callbacks
 * @param signal     Optional external AbortSignal for cancellation
 * @returns An `abort` function for cancelling the stream
 */
export function streamChat(
  sessionId: string,
  message: string,
  handlers: ChatStreamHandlers,
  signal?: AbortSignal,
): { abort: () => void } {
  const controller = new AbortController()
  const combinedSignal = signal ? mergeAbortSignals(signal, controller.signal) : controller.signal

  // Shared across then/catch so connection-level errors can compute a seq
  // that passes the consumer-side monotonic-seq check.
  let eventSeq = 0

  // Build request body per CreateQAMessageRequest
  const body: Record<string, unknown> = { message }

  fetch(`${apiClient.baseUrl}/qa-sessions/${encodeURIComponent(sessionId)}/messages`, {
    method: 'POST',
    headers: buildStreamHeaders(),
    body: JSON.stringify(body),
    signal: combinedSignal,
  })
    .then(async (res) => {
      if (!res.ok) {
        const text = await res.text().catch(() => '')
        handlers.onError?.({
          code: String(res.status),
          message: text || '请求失败',
          fatal: true,
          seq: 0,
        })
        return
      }

      const reader = res.body?.getReader()
      if (!reader) {
        handlers.onError?.({
          code: 'no_body',
          message: '无法读取响应流',
          fatal: true,
          seq: 0,
        })
        return
      }

      const decoder = new TextDecoder()
      let buffer = ''
      // currentEvent persists across read-loop iterations so that when an
      // SSE event is split across network chunks the data: line in the
      // later chunk still sees the event type from the earlier chunk.
      let currentEvent: string | null = null
      let currentData: string | null = null

      const flushEvent = () => {
        if (!currentEvent || currentData === null) return
        eventSeq++
        try {
          const raw: Record<string, unknown> = JSON.parse(currentData)
          const data = { seq: eventSeq, ...raw } as unknown
          dispatch(currentEvent as QASseEventType, data, handlers)
        } catch {
          // ignore unparseable data lines
        }
        currentEvent = null
        currentData = null
      }

      const processLines = (lines: string[]) => {
        for (const line of lines) {
          // Strip trailing CR for cross-platform compatibility
          const trimmed = line.endsWith('\r') ? line.slice(0, -1) : line

          if (trimmed === '') {
            // Blank line — SSE event boundary
            flushEvent()
          } else if (trimmed.startsWith('event: ')) {
            // New event type — flush previous event (if any), then capture
            flushEvent()
            currentEvent = trimmed.slice(7).trim()
          } else if (trimmed.startsWith('data: ')) {
            currentData = trimmed.slice(6)
          }
          // Lines starting with ':' are SSE comments — silently ignored
        }
      }

      for (;;) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        // Last element may be a partial line; save it for the next read
        buffer = lines.pop() || ''

        processLines(lines)
      }

      // Flush decoder remainder + any buffered partial line
      buffer += decoder.decode()
      if (buffer.trim()) {
        processLines(buffer.split('\n'))
      }
      // Flush any event that was fully received before stream end
      flushEvent()
    })
    .catch((err) => {
      if (err instanceof DOMException && err.name === 'AbortError') {
        handlers.onAbort?.()
        return
      }
      // Connection-level errors use seq = eventSeq + 1 so they always pass
      // the consumer-side monotonic-seq check, even when events have already
      // been dispatched.
      handlers.onError?.({
        code: 'connection_error',
        message: err instanceof Error ? err.message : '网络异常，请检查连接',
        fatal: true,
        seq: eventSeq + 1,
      })
    })

  return { abort: () => controller.abort() }
}

// ---------------------------------------------------------------------------
// POST /qa-sessions/{sessionId}/messages  (non-streaming)
// ---------------------------------------------------------------------------

/**
 * Send a chat message and receive a completed QAMessage response.
 *
 * Unlike `streamChat`, this does NOT set `Accept: text/event-stream`.
 * The server returns a single JSON response containing the created QAMessage.
 *
 * @param sessionId  QA session id (path parameter)
 * @param message    User message text (required body field)
 * @returns The created QAMessage
 */
export async function sendMessage(sessionId: string, message: string): Promise<QAMessage> {
  return gatewayRequest<QAMessage>(`/qa-sessions/${encodeURIComponent(sessionId)}/messages`, {
    method: 'POST',
    body: { message },
  })
}

// ---------------------------------------------------------------------------
// GET /qa-sessions/{sessionId}/events?responseRunId=...
// ---------------------------------------------------------------------------

/**
 * Replay short-lived persisted SSE events for reconnect recovery or debugging.
 *
 * Polls the server for events associated with a specific response run.
 * Useful after a disconnect to recover missed streaming events.
 *
 * @param sessionId      QA session id (path parameter)
 * @param responseRunId  The response run to replay events for (query parameter)
 * @returns Array of QASseEvent objects
 */
export async function replayEvents(
  sessionId: string,
  responseRunId: string,
): Promise<QASseEvent[]> {
  return gatewayRequest<QASseEvent[]>(
    `/qa-sessions/${encodeURIComponent(sessionId)}/events${buildQuery({ responseRunId })}`,
  )
}

// ---------------------------------------------------------------------------
// POST /knowledge-queries
// ---------------------------------------------------------------------------

/**
 * Run a knowledge-base retrieval query without LLM.
 * Replaces the legacy RAG search endpoint.
 */
export async function queryKnowledge(
  params: KnowledgeQueryRequest,
): Promise<KnowledgeQuerySummary> {
  return gatewayRequest<KnowledgeQuerySummary>('/knowledge-queries', {
    method: 'POST',
    body: params,
  })
}
