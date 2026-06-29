/**
 * React Query hooks for session CRUD.
 *
 * Server state managed by TanStack Query; UI cache synchronised via
 * the Zustand chat store on mutation success.
 */

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import {
  createSession,
  deleteSession,
  getSession,
  getSessionMessages,
  listSessions,
  renameSession,
} from '@/api/conversations'

// ── Query keys ──

export const sessionKeys = {
  all: ['sessions'] as const,
  lists: () => [...sessionKeys.all, 'list'] as const,
  list: (page: number, pageSize: number) => [...sessionKeys.lists(), { page, pageSize }] as const,
  details: () => [...sessionKeys.all, 'detail'] as const,
  detail: (id: string) => [...sessionKeys.details(), id] as const,
  messages: (id: string) => [...sessionKeys.details(), id, 'messages'] as const,
}

// ── Queries ──

/** Paginated session list. */
export function useSessions(page = 1, pageSize = 20) {
  return useQuery({
    queryKey: sessionKeys.list(page, pageSize),
    queryFn: () => listSessions({ page, pageSize }),
    placeholderData: (prev) => prev,
  })
}

/** Single session detail (includes messages). */
export function useSession(id: string) {
  return useQuery({
    queryKey: sessionKeys.detail(id),
    queryFn: () => getSession(id),
    enabled: id.length > 0,
  })
}

/** Messages for a specific session (standalone endpoint). */
export function useSessionMessages(sessionId: string) {
  return useQuery({
    queryKey: sessionKeys.messages(sessionId),
    queryFn: () => getSessionMessages(sessionId),
    enabled: sessionId.length > 0,
  })
}

// ── Mutations ──

/** Create a new session. */
export function useCreateSession() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (title?: string) => createSession(title),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: sessionKeys.lists(),
      })
    },
  })
}

/** Delete a session. */
export function useDeleteSession() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => deleteSession(id),
    onSuccess: (_data, id) => {
      void queryClient.invalidateQueries({
        queryKey: sessionKeys.lists(),
      })
      queryClient.removeQueries({
        queryKey: sessionKeys.detail(id),
      })
      queryClient.removeQueries({
        queryKey: sessionKeys.messages(id),
      })
    },
  })
}

/** Rename a session. */
export function useRenameSession() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ sessionId, title }: { sessionId: string; title: string }) =>
      renameSession(sessionId, title),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({
        queryKey: sessionKeys.lists(),
      })
      void queryClient.invalidateQueries({
        queryKey: sessionKeys.detail(variables.sessionId),
      })
    },
  })
}
