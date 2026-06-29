/**
 * Chat UI state — sessions metadata, per-session messages, streaming flag, error tracking.
 *
 * QASession does NOT embed messages — they are stored separately in `messagesBySession`.
 * Only session IDs are persisted to localStorage so the sidebar can
 * restore the session list across page reloads.
 */

import { create } from 'zustand'
import { persist } from 'zustand/middleware'

import type { QAMessage, QASession } from '@/lib/types'

export interface ChatState {
  /** Full session metadata objects (in-memory, fetched from server or created locally). */
  sessions: QASession[]
  /** Session IDs persisted to localStorage for session recovery. */
  sessionIds: string[]
  /** Currently selected session. */
  activeId: string | null
  /** Whether an SSE stream is in progress. */
  streaming: boolean
  /** Last fatal error message for display. */
  error: string | null
  /** The user message that triggered a fatal error (for retry). */
  lastFailedMsg: string | null
  /** Messages keyed by sessionId (QASession does not embed messages). */
  messagesBySession: Record<string, QAMessage[]>

  // ── Actions ──

  /** Bulk-set session metadata (used when syncing from server). */
  setSessions: (sessions: QASession[]) => void
  setSessionIds: (ids: string[]) => void
  setActiveId: (id: string | null) => void
  /** Prepend a new session metadata, deduping by sessionId. Also updates persisted sessionIds. */
  addSession: (session: QASession) => void
  /** Remove a session, its messages, and its persisted id. Clears activeId if it matches. */
  removeSession: (sessionId: string) => void
  /** Replace the messages array for a given session. */
  updateSessionMessages: (sessionId: string, messages: QAMessage[]) => void
  /** Prepend a new message to a session's message list. */
  appendSessionMessages: (sessionId: string, messages: QAMessage[]) => void
  setStreaming: (streaming: boolean) => void
  setError: (error: string | null) => void
  setLastFailedMsg: (msg: string | null) => void
  clearError: () => void
}

export const useChatStore = create<ChatState>()(
  persist(
    (set) => ({
      sessions: [],
      sessionIds: [],
      activeId: null,
      streaming: false,
      error: null,
      lastFailedMsg: null,
      messagesBySession: {},

      setSessions: (sessions) => set({ sessions }),

      setSessionIds: (ids) => set({ sessionIds: ids }),

      setActiveId: (id) => set({ activeId: id }),

      addSession: (session) =>
        set((state) => {
          if (state.sessions.some((s) => s.id === session.id)) {
            return state
          }
          return {
            sessions: [session, ...state.sessions],
            sessionIds: [session.id, ...state.sessionIds.filter((sid) => sid !== session.id)],
          }
        }),

      removeSession: (sessionId) =>
        set((state) => {
          const { [sessionId]: _removed, ...restMessages } = state.messagesBySession
          return {
            sessions: state.sessions.filter((s) => s.id !== sessionId),
            sessionIds: state.sessionIds.filter((sid) => sid !== sessionId),
            activeId: state.activeId === sessionId ? null : state.activeId,
            messagesBySession: restMessages,
          }
        }),

      updateSessionMessages: (sessionId, messages) =>
        set((state) => ({
          messagesBySession: {
            ...state.messagesBySession,
            [sessionId]: messages,
          },
        })),

      appendSessionMessages: (sessionId, messages) =>
        set((state) => ({
          messagesBySession: {
            ...state.messagesBySession,
            [sessionId]: [
              ...(state.messagesBySession[sessionId] ?? []),
              ...messages,
            ],
          },
        })),

      setStreaming: (streaming) => set({ streaming }),

      setError: (error) => set({ error }),

      setLastFailedMsg: (msg) => set({ lastFailedMsg: msg }),

      clearError: () => set({ error: null, lastFailedMsg: null }),
    }),
    {
      name: 'qa-sessions-ids',
      partialize: (state) => ({ sessionIds: state.sessionIds }),
    },
  ),
)
