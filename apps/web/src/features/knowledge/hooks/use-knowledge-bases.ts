/**
 * React Query hooks for knowledge base CRUD.
 *
 * Server state managed by TanStack Query with client-side
 * keyword/docType filtering and pagination.
 */

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import {
  createKnowledgeBase,
  deleteKnowledgeBase,
  getKnowledgeBase,
  listKnowledgeBases,
  updateKnowledgeBase,
} from '@/api/admin'
import type { UpdateKnowledgeBaseRequest } from '@/lib/types'

// ── Query keys ──

export const knowledgeBaseKeys = {
  all: ['knowledge-bases'] as const,
  lists: () => [...knowledgeBaseKeys.all, 'list'] as const,
  list: (page: number, pageSize: number, keyword?: string, docType?: string) =>
    [...knowledgeBaseKeys.lists(), { page, pageSize, keyword, docType }] as const,
  details: () => [...knowledgeBaseKeys.all, 'detail'] as const,
  detail: (id: string) => [...knowledgeBaseKeys.details(), id] as const,
}

// ── Queries ──

/**
 * Knowledge base list with server-side pagination and client-side keyword/docType filtering.
 *
 * Keyword and docType filtering is client-side per-page since the backend
 * list endpoint does not currently expose search/filter query parameters.
 * Server total is used for accurate page count.
 *
 * **Limitation**: Filtering is applied only to the current page of results
 * fetched from the server. When a keyword or docType filter is active,
 * filtered results may be incomplete -- items matching the filter on other
 * pages are not included. This is indicated by the `filteredLocally` flag
 * in the return value, which is `true` whenever a filter is active and the
 * current page may not represent the full filtered dataset.
 */
export function useKnowledgeBases(page = 1, pageSize = 10, keyword?: string, docType?: string) {
  const hasFilter = Boolean(keyword || docType)

  return useQuery({
    queryKey: knowledgeBaseKeys.list(page, pageSize, keyword, docType),
    queryFn: async () => {
      const result = await listKnowledgeBases({ page, pageSize })
      let items = result.items

      if (keyword) {
        const kw = keyword.toLowerCase()
        items = items.filter(
          (kb) =>
            kb.name.toLowerCase().includes(kw) ||
            (kb.description && kb.description.toLowerCase().includes(kw)),
        )
      }

      if (docType) {
        items = items.filter((kb) => kb.docType === docType)
      }

      return {
        items,
        page: { page: result.page.page, pageSize: result.page.pageSize, total: result.page.total },
        filteredLocally: hasFilter,
      }
    },
    placeholderData: (prev) => prev,
  })
}

/** Single knowledge base detail. */
export function useKnowledgeBase(id: string) {
  return useQuery({
    queryKey: knowledgeBaseKeys.detail(id),
    queryFn: () => getKnowledgeBase(id),
    enabled: id.length > 0,
  })
}

// ── Mutations ──

/** Create a new knowledge base. */
export function useCreateKnowledgeBase() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: createKnowledgeBase,
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: knowledgeBaseKeys.lists(),
      })
    },
  })
}

/** Update an existing knowledge base. */
export function useUpdateKnowledgeBase() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ id, ...params }: { id: string } & UpdateKnowledgeBaseRequest) =>
      updateKnowledgeBase(id, params),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({
        queryKey: knowledgeBaseKeys.lists(),
      })
      void queryClient.invalidateQueries({
        queryKey: knowledgeBaseKeys.detail(variables.id),
      })
    },
  })
}

/** Delete a knowledge base. */
export function useDeleteKnowledgeBase() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => deleteKnowledgeBase(id),
    onSuccess: (_data, id) => {
      void queryClient.invalidateQueries({
        queryKey: knowledgeBaseKeys.lists(),
      })
      queryClient.removeQueries({
        queryKey: knowledgeBaseKeys.detail(id),
      })
    },
  })
}
