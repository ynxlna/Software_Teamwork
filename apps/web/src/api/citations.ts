/**
 * Citation endpoints — Gateway OpenAPI qa-citations paths.
 *
 * getCitation      GET  /citations/{citationId}
 * lookupCitations  POST /citation-lookups
 */

import type { QACitationDetail } from '@/lib/types'

import { gatewayRequest } from './client'

// ---------------------------------------------------------------------------
// GET /citations/{citationId}
// ---------------------------------------------------------------------------

/**
 * Retrieve full citation detail including source availability
 * for a single citation.
 */
export async function getCitation(citationId: string): Promise<QACitationDetail> {
  return gatewayRequest<QACitationDetail>(`/citations/${encodeURIComponent(citationId)}`)
}

// ---------------------------------------------------------------------------
// POST /citation-lookups
// ---------------------------------------------------------------------------

/**
 * Batch lookup citation details by citation IDs.
 */
export async function lookupCitations(citationIds: string[]): Promise<QACitationDetail[]> {
  return gatewayRequest<QACitationDetail[]>('/citation-lookups', {
    method: 'POST',
    body: JSON.stringify({ citationIds }),
  })
  return citations.map(toCitationDetail)
}

export type { BatchCitationsRequest }
