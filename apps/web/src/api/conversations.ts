/**
 * QA Sessions CRUD — Gateway OpenAPI qa-sessions paths.
 *
 * All functions use gatewayRequest / gatewayPageRequest from ./client.
 * Types imported from @/lib/types (re-exported from generated/gateway).
 */

import type { QAMessage, QASession, QASessionStatus } from '@/lib/types'

import { buildQuery, gatewayPageRequest, gatewayRequest } from './client'

// ---------------------------------------------------------------------------
// POST /qa-sessions
// ---------------------------------------------------------------------------

export async function createSession(title?: string): Promise<QASession> {
  return gatewayRequest<QASession>('/qa-sessions', {
    method: 'POST',
    body: JSON.stringify({ title }),
  })
}

// ---------------------------------------------------------------------------
// GET /qa-sessions?page=&pageSize=&status=&sort=-updatedAt
// ---------------------------------------------------------------------------

export interface ListSessionsParams {
  page?: number
  pageSize?: number
  status?: QASessionStatus
  sort?: string
}

export async function listSessions(
  params: ListSessionsParams = {},
): Promise<{ items: QASession[]; page: { page: number; pageSize: number; total: number } }> {
  return gatewayPageRequest<QASession>(
    `/qa-sessions${buildQuery({
      page: params.page,
      pageSize: params.pageSize,
      status: params.status,
      sort: params.sort ?? '-updatedAt',
    })}`,
  )
}

// ---------------------------------------------------------------------------
// GET /qa-sessions/{sessionId}
// ---------------------------------------------------------------------------

export async function getSession(sessionId: string): Promise<QASession> {
  return gatewayRequest<QASession>(`/qa-sessions/${encodeURIComponent(sessionId)}`)
}

// ---------------------------------------------------------------------------
// PATCH /qa-sessions/{sessionId}
// ---------------------------------------------------------------------------

export async function renameSession(sessionId: string, title: string): Promise<QASession> {
  return gatewayRequest<QASession>(`/qa-sessions/${encodeURIComponent(sessionId)}`, {
    method: 'PATCH',
    body: { title },
  })
  return toConversation(session)
}

// ---------------------------------------------------------------------------
// DELETE /qa-sessions/{sessionId}
// ---------------------------------------------------------------------------

export async function deleteSession(sessionId: string): Promise<void> {
  await gatewayRequest<void>(`/qa-sessions/${encodeURIComponent(sessionId)}`, { method: 'DELETE' })
}

// ---------------------------------------------------------------------------
// GET /qa-sessions/{sessionId}/messages?page=&pageSize=&includeThinking=&includeCitations=
// ---------------------------------------------------------------------------

export interface GetSessionMessagesParams {
  page?: number
  pageSize?: number
  includeThinking?: boolean
  includeCitations?: boolean
}

export async function getSessionMessages(
  sessionId: string,
  params: GetSessionMessagesParams = {},
): Promise<{ items: QAMessage[]; page: { page: number; pageSize: number; total: number } }> {
  return gatewayPageRequest<QAMessage>(
    `/qa-sessions/${encodeURIComponent(sessionId)}/messages${buildQuery({
      page: params.page,
      pageSize: params.pageSize,
      includeThinking: params.includeThinking,
      includeCitations: params.includeCitations,
    })}`,
  )
}
