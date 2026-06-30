/**
 * React Query hooks for model profile CRUD.
 *
 * Server state managed by TanStack Query.
 * apiKey is write-only — the response only indicates apiKeyConfigured: boolean.
 */

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import {
  createModelProfile,
  deleteModelProfile,
  getModelProfile,
  listModelProfiles,
  updateModelProfile,
} from '@/api/admin'
import type { CreateModelProfileRequest, UpdateModelProfileRequest } from '@/lib/types'

// ── Query keys ──

export const modelProfileKeys = {
  all: ['admin', 'model-profiles'] as const,
  lists: () => [...modelProfileKeys.all, 'list'] as const,
  list: (purpose?: string, enabled?: boolean) =>
    [...modelProfileKeys.lists(), { purpose, enabled }] as const,
  details: () => [...modelProfileKeys.all, 'detail'] as const,
  detail: (id: string) => [...modelProfileKeys.details(), id] as const,
}

// ── Queries ──

/** List all runtime model profiles (non-paginated). */
export function useModelProfiles(purpose?: string, enabled?: boolean) {
  return useQuery({
    queryKey: modelProfileKeys.list(purpose, enabled),
    queryFn: () => listModelProfiles({ purpose, enabled }),
  })
}

/** Single model profile detail. */
export function useModelProfile(id: string) {
  return useQuery({
    queryKey: modelProfileKeys.detail(id),
    queryFn: () => getModelProfile(id),
    enabled: id.length > 0,
  })
}

// ── Mutations ──

/** Create a new model profile. */
export function useCreateModelProfile() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (params: CreateModelProfileRequest) => createModelProfile(params),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: modelProfileKeys.lists(),
      })
    },
  })
}

/** Update an existing model profile. */
export function useUpdateModelProfile() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ id, ...params }: { id: string } & UpdateModelProfileRequest) =>
      updateModelProfile(id, params),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({
        queryKey: modelProfileKeys.lists(),
      })
      void queryClient.invalidateQueries({
        queryKey: modelProfileKeys.detail(variables.id),
      })
    },
  })
}

/** Delete a model profile. */
export function useDeleteModelProfile() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => deleteModelProfile(id),
    onSuccess: (_data, id) => {
      void queryClient.invalidateQueries({
        queryKey: modelProfileKeys.lists(),
      })
      queryClient.removeQueries({
        queryKey: modelProfileKeys.detail(id),
      })
    },
  })
}
