/**
 * React Query hooks for document CRUD, chunks, content, and knowledge search.
 *
 * Server state managed by TanStack Query with client-side caching and optimisations.
 */

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import {
  deleteDocument,
  getDocument,
  getDocumentContent,
  listChunks,
  listDocuments,
  runKnowledgeQuery,
  updateDocument,
  uploadDocument,
} from '@/api/admin'
import type { KnowledgeQueryRequest, UpdateDocumentRequest } from '@/lib/types'

// ── Query keys ──

export const documentKeys = {
  all: ['documents'] as const,
  lists: () => [...documentKeys.all, 'list'] as const,
  list: (knowledgeBaseId: string, page: number, pageSize: number, status?: string) =>
    [...documentKeys.lists(), { knowledgeBaseId, page, pageSize, status }] as const,
  details: () => [...documentKeys.all, 'detail'] as const,
  detail: (id: string) => [...documentKeys.details(), id] as const,
  chunks: (documentId: string) => [...documentKeys.all, 'chunks', documentId] as const,
  chunkPage: (documentId: string, page: number, pageSize: number) =>
    [...documentKeys.chunks(documentId), { page, pageSize }] as const,
  content: (documentId: string) => [...documentKeys.all, 'content', documentId] as const,
  search: ['knowledge-search'] as const,
}

// ── Queries ──

/** Paginated document list for a knowledge base. */
export function useDocuments(knowledgeBaseId: string, page = 1, pageSize = 20, status?: string) {
  return useQuery({
    queryKey: documentKeys.list(knowledgeBaseId, page, pageSize, status),
    queryFn: () => listDocuments(knowledgeBaseId, { page, pageSize, status }),
    placeholderData: (prev) => prev,
    enabled: Boolean(knowledgeBaseId),
  })
}

/** Single document detail. */
export function useDocument(id: string) {
  return useQuery({
    queryKey: documentKeys.detail(id),
    queryFn: () => getDocument(id),
    enabled: id.length > 0,
  })
}

/** Paginated document chunks. */
export function useChunks(documentId: string, page = 1, pageSize = 50) {
  return useQuery({
    queryKey: documentKeys.chunkPage(documentId, page, pageSize),
    queryFn: () => listChunks(documentId, { page, pageSize }),
    placeholderData: (prev) => prev,
    enabled: Boolean(documentId),
  })
}

/** Document raw content as Blob (for download). */
export function useDocumentContent(documentId: string) {
  return useQuery({
    queryKey: documentKeys.content(documentId),
    queryFn: () => getDocumentContent(documentId),
    enabled: Boolean(documentId),
    staleTime: Infinity,
  })
}

// ── Mutations ──

/** Upload a document to a knowledge base. */
export function useUploadDocument() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({
      knowledgeBaseId,
      file,
      tags,
    }: {
      knowledgeBaseId: string
      file: File
      tags?: string[]
    }) => uploadDocument(knowledgeBaseId, file, tags),
    onSuccess: (_data, _variables) => {
      void queryClient.invalidateQueries({
        queryKey: documentKeys.lists(),
      })
      void queryClient.invalidateQueries({
        queryKey: ['knowledge-bases'],
      })
    },
  })
}

/** Update document metadata (tags). */
export function useUpdateDocument() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ id, ...params }: { id: string } & UpdateDocumentRequest) =>
      updateDocument(id, params),
    onSuccess: (_data, _variables) => {
      void queryClient.invalidateQueries({
        queryKey: documentKeys.lists(),
      })
      void queryClient.invalidateQueries({
        queryKey: documentKeys.detail(_variables.id),
      })
    },
  })
}

/** Delete a document. */
export function useDeleteDocument() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => deleteDocument(id),
    onSuccess: (_data, id) => {
      void queryClient.invalidateQueries({
        queryKey: documentKeys.lists(),
      })
      queryClient.removeQueries({
        queryKey: documentKeys.detail(id),
      })
      void queryClient.invalidateQueries({
        queryKey: ['knowledge-bases'],
      })
    },
  })
}

/** Run a knowledge retrieval query (search). */
export function useKnowledgeSearch() {
  return useMutation({
    mutationFn: (params: KnowledgeQueryRequest) => runKnowledgeQuery(params),
  })
}
