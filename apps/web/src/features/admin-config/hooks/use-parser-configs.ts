/**
 * React Query hooks for parser config CRUD.
 *
 * Server state managed by TanStack Query.
 */

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import {
  createParserConfig,
  deleteParserConfig,
  getParserConfig,
  listParserConfigs,
  updateParserConfig,
} from '@/api/admin'
import type { CreateParserConfigRequest, UpdateParserConfigRequest } from '@/lib/types'

// ── Query keys ──

export const parserConfigKeys = {
  all: ['admin', 'parser-configs'] as const,
  lists: () => [...parserConfigKeys.all, 'list'] as const,
  list: (enabled?: boolean) => [...parserConfigKeys.lists(), { enabled }] as const,
  details: () => [...parserConfigKeys.all, 'detail'] as const,
  detail: (id: string) => [...parserConfigKeys.details(), id] as const,
}

// ── Queries ──

/** List all document parser configs (non-paginated). */
export function useParserConfigs(enabled?: boolean) {
  return useQuery({
    queryKey: parserConfigKeys.list(enabled),
    queryFn: () => listParserConfigs({ enabled }),
  })
}

/** Single parser config detail. */
export function useParserConfig(id: string) {
  return useQuery({
    queryKey: parserConfigKeys.detail(id),
    queryFn: () => getParserConfig(id),
    enabled: id.length > 0,
  })
}

// ── Mutations ──

/** Create a new parser config. */
export function useCreateParserConfig() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (params: CreateParserConfigRequest) => createParserConfig(params),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: parserConfigKeys.lists(),
      })
    },
  })
}

/** Update an existing parser config. */
export function useUpdateParserConfig() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ id, ...params }: { id: string } & UpdateParserConfigRequest) =>
      updateParserConfig(id, params),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({
        queryKey: parserConfigKeys.lists(),
      })
      void queryClient.invalidateQueries({
        queryKey: parserConfigKeys.detail(variables.id),
      })
    },
  })
}

/** Delete a parser config. */
export function useDeleteParserConfig() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => deleteParserConfig(id),
    onSuccess: (_data, id) => {
      void queryClient.invalidateQueries({
        queryKey: parserConfigKeys.lists(),
      })
      queryClient.removeQueries({
        queryKey: parserConfigKeys.detail(id),
      })
    },
  })
}
