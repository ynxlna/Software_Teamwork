/**
 * Chat UI state — sessions cache, streaming flag, error tracking.
 *
 * Full session objects live in memory only (they come from the server).
 * Only session IDs are persisted to localStorage so the sidebar can
 * restore the session list across page reloads.
 */

import { create } from 'zustand'
import { persist } from 'zustand/middleware'

import type { Conversation, Message } from '@/lib/types'

export interface ChatState {
  /** Full session objects (in-memory, fetched from server or created locally). */
  sessions: Conversation[]
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

  // ── Actions ──

  /** Bulk-set sessions (used when syncing from server). */
  setSessions: (sessions: Conversation[]) => void
  setSessionIds: (ids: string[]) => void
  setActiveId: (id: string | null) => void
  /** Prepend a new session, deduping by id. Also updates persisted sessionIds. */
  addSession: (session: Conversation) => void
  /** Remove a session and its persisted id. Clears activeId if it matches. */
  removeSession: (id: string) => void
  /** Replace the messages array for a given session. */
  updateSessionMessages: (id: string, messages: Message[]) => void
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
            sessionIds: [
              session.id,
              ...state.sessionIds.filter((sid) => sid !== session.id),
            ],
          }
        }),

      removeSession: (id) =>
        set((state) => ({
          sessions: state.sessions.filter((s) => s.id !== id),
          sessionIds: state.sessionIds.filter((sid) => sid !== id),
          activeId: state.activeId === id ? null : state.activeId,
        })),

      updateSessionMessages: (id, messages) =>
        set((state) => ({
          sessions: state.sessions.map((s) =>
            s.id === id ? { ...s, messages } : s,
          ),
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
