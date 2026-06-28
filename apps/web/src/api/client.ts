/**
 * Base API client with error handling.
 *
 * Uses native fetch (no axios).
 * code/message/data response pattern per API doc section 1.
 */

export class ApiError extends Error {
  code: number

  constructor(code: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.code = code
  }
}

/** Resolved at build time by Vite; falls back to same-origin `/api`. */
export const apiClient = {
  baseUrl: import.meta.env?.VITE_API_BASE_URL ?? '/api/v1',
}

interface ApiEnvelope<T> {
  code: number
  message: string
  data: T
}

/**
 * Generic request helper that handles the unified { code, message, data }
 * envelope.  Throws `ApiError` for non-0 code or non-OK HTTP status.
 */
export async function doRequest<T>(
  path: string,
  options?: RequestInit,
): Promise<T> {
  const url = `${apiClient.baseUrl}${path}`
  const res = await fetch(url, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
  })

  if (!res.ok) {
    throw new ApiError(res.status, `HTTP ${res.status}: ${res.statusText}`)
  }

  const json: ApiEnvelope<T> = await res.json()
  if (json.code !== 0) {
    throw new ApiError(json.code, json.message)
  }

  return json.data
}
