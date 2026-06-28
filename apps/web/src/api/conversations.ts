/**
 * Session (qa-sessions) CRUD — API doc section 2.
 *
 * All functions use the doRequest helper for unified { code, message, data }
 * error handling.  File name kept as conversations.ts for compatibility;
 * internal naming uses "session" / "sessions".
 */

import type { Conversation, ConversationListItem, Message } from '@/lib/types'

import { doRequest } from './client'

// ---------------------------------------------------------------------------
// 2.1  Create session
// ---------------------------------------------------------------------------

export async function createSession(
  title = '新对话',
): Promise<Conversation> {
  return doRequest<Conversation>('/qa-sessions', {
    method: 'POST',
    body: JSON.stringify({ title }),
  })
}

// ---------------------------------------------------------------------------
// 2.2  List sessions (paginated)
// ---------------------------------------------------------------------------

export async function listSessions(
  page = 1,
  pageSize = 20,
): Promise<{ items: ConversationListItem[]; total: number }> {
  const params = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
    sort: 'updated_at_desc',
  })
  return doRequest<{ items: ConversationListItem[]; total: number }>(
    `/qa-sessions?${params}`,
  )
}

// ---------------------------------------------------------------------------
// 2.3  Get session detail
// ---------------------------------------------------------------------------

export async function getSession(
  id: string,
): Promise<Conversation> {
  return doRequest<Conversation>(
    `/qa-sessions/${encodeURIComponent(id)}`,
  )
}

// ---------------------------------------------------------------------------
// 2.4  Rename session (PATCH)
// ---------------------------------------------------------------------------

export async function renameSession(
  sessionId: string,
  title: string,
): Promise<Conversation> {
  return doRequest<Conversation>(
    `/qa-sessions/${encodeURIComponent(sessionId)}`,
    {
      method: 'PATCH',
      body: JSON.stringify({ title }),
    },
  )
}

// ---------------------------------------------------------------------------
// 2.5  Delete session
// ---------------------------------------------------------------------------

export async function deleteSession(id: string): Promise<void> {
  await doRequest<void>(
    `/qa-sessions/${encodeURIComponent(id)}`,
    { method: 'DELETE' },
  )
}

// ---------------------------------------------------------------------------
// 2.6  Get session messages
// ---------------------------------------------------------------------------

export async function getSessionMessages(
  sessionId: string,
): Promise<Message[]> {
  return doRequest<Message[]>(
    `/qa-sessions/${encodeURIComponent(sessionId)}/messages`,
  )
}
