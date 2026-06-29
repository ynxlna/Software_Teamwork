/**
 * API client aligned with the Gateway OpenAPI specification.
 *
 * Envelope contracts:
 * - Success  : { data: T, requestId: string }
 * - List     : { data: T[], page: { page, pageSize, total }, requestId: string }
 * - Error    : { error: { code: string, message: string, requestId: string, fields?: Record<string,string> } }
 *
 * Auth       : Authorization: Bearer <token> on every business call.
 * Health     : GET /healthz and GET /readyz — no auth required.
 */

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface PageInfo {
  page: number
  pageSize: number
  total: number
}

export interface ListResponse<T> {
  items: T[]
  page: PageInfo
}

// ---------------------------------------------------------------------------
// Error
// ---------------------------------------------------------------------------

export class ApiError extends Error {
  code: string
  requestId: string
  fields?: Record<string, string>

  constructor(code: string, message: string, requestId: string, fields?: Record<string, string>) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.requestId = requestId
    this.fields = fields
  }
}

// ---------------------------------------------------------------------------
// Envelope shapes (internal)
// ---------------------------------------------------------------------------

interface SuccessEnvelope<T> {
  data: T
  requestId: string
}

interface ListEnvelope<T> {
  data: T[]
  page: PageInfo
  requestId: string
}

interface ErrorEnvelope {
  error: {
    code: string
    message: string
    requestId: string
    fields?: Record<string, string>
  }
}

// ---------------------------------------------------------------------------
// Client
// ---------------------------------------------------------------------------

const AUTH_TOKEN_KEY = 'auth_token'

class ApiClientImpl {
  baseUrl: string
  private token: string | null = null

  constructor() {
    this.baseUrl =
      ((import.meta as Record<string, unknown>).env?.VITE_API_BASE_URL as string | undefined) ??
      '/api/v1'

    // Restore token from localStorage on init
    try {
      const stored = localStorage.getItem(AUTH_TOKEN_KEY)
      if (stored) this.token = stored
    } catch {
      // localStorage may be unavailable (SSR, test env)
    }
  }

  // ── Token management ──

  getToken(): string | null {
    return this.token
  }

  setToken(token: string | null): void {
    this.token = token
    if (token) {
      try {
        localStorage.setItem(AUTH_TOKEN_KEY, token)
      } catch {
        // noop
      }
    } else {
      try {
        localStorage.removeItem(AUTH_TOKEN_KEY)
      } catch {
        // noop
      }
    }
  }

  // ── Request helpers ──

  /**
   * Single-resource request.
   * Parses `{ data, requestId }` on 2xx → returns `data`.
   * Parses `{ error }` on non-2xx → throws `ApiError`.
   * Clears token on 401.
   * @deprecated Use `gatewayRequest` instead. This class-based method is kept for
   * backward compatibility with existing API modules during migration.
   */
  async doRequest<T>(path: string, options?: RequestInit): Promise<T> {
    const res = await this.fetchWithAuth(path, options)

    if (res.status === 401) {
      this.setToken(null)
    }

    if (!res.ok) {
      throw await this.parseError(res)
    }

    // 204 No Content
    if (res.status === 204) {
      return undefined as T
    }

    const json: SuccessEnvelope<T> = (await res.json()) as SuccessEnvelope<T>
    return json.data
  }

  /**
   * Paginated-list request.
   * Parses `{ data, page, requestId }` on 2xx.
   * @deprecated Use `gatewayPageRequest` instead. This class-based method is kept for
   * backward compatibility with existing API modules during migration.
   */
  async listRequest<T>(path: string, options?: RequestInit): Promise<ListResponse<T>> {
    const res = await this.fetchWithAuth(path, options)

    if (res.status === 401) {
      this.setToken(null)
    }

    if (!res.ok) {
      throw await this.parseError(res)
    }

    const json: ListEnvelope<T> = (await res.json()) as ListEnvelope<T>
    return {
      items: json.data,
      page: json.page,
    }
  }

  /**
   * Raw fetch that returns the Response object for binary downloads, etc.
   * Does NOT parse the envelope — caller is responsible for reading body.
   */
  async rawRequest(path: string, options?: RequestInit): Promise<Response> {
    const res = await this.fetchWithAuth(path, options)

    if (res.status === 401) {
      this.setToken(null)
    }

    if (!res.ok) {
      throw await this.parseError(res)
    }

    return res
  }

  // ── Health (no auth) ──

  async healthz(): Promise<{ status: string }> {
    const res = await fetch(`${this.baseUrl}/healthz`)
    if (!res.ok) {
      throw new ApiError('health_check_failed', 'Health check failed', '')
    }
    const json = (await res.json()) as SuccessEnvelope<{ status: string }>
    return json.data
  }

  async readyz(): Promise<{ status: string }> {
    const res = await fetch(`${this.baseUrl}/readyz`)
    if (!res.ok) {
      throw new ApiError('readiness_check_failed', 'Readiness check failed', '')
    }
    const json = (await res.json()) as SuccessEnvelope<{ status: string }>
    return json.data
  }

  // ── Internals ──

  private async fetchWithAuth(path: string, options?: RequestInit): Promise<Response> {
    const headers = new Headers(options?.headers)

    // Ensure Content-Type for requests with a body (unless it's FormData)
    const hasBody = options?.body != null && !(options.body instanceof FormData)
    if (hasBody && !headers.has('Content-Type')) {
      headers.set('Content-Type', 'application/json')
    }

    // Attach auth token for business API calls
    if (this.token) {
      headers.set('Authorization', `Bearer ${this.token}`)
    }

    return fetch(`${this.baseUrl}${path}`, {
      ...options,
      headers,
    })
  }

  private async parseError(res: Response): Promise<ApiError> {
    try {
      const json: ErrorEnvelope = (await res.json()) as ErrorEnvelope
      return new ApiError(
        json.error.code,
        json.error.message,
        json.error.requestId,
        json.error.fields,
      )
    } catch {
      return new ApiError('http_error', `HTTP ${res.status}: ${res.statusText}`, '')
    }
  }
}

/** Singleton API client instance. */
export const apiClient = new ApiClientImpl()

// ---------------------------------------------------------------------------
// Standalone function exports — convenience wrappers for API module imports.
// Usage: import { doRequest, listRequest } from './client'
// ---------------------------------------------------------------------------

/**
 * Single-resource request via the singleton API client.
 * @see ApiClientImpl.doRequest
 * @deprecated Use `gatewayRequest` instead. This wrapper is kept for backward
 * compatibility with existing API modules during migration.
 */
export function doRequest<T>(path: string, options?: RequestInit): Promise<T> {
  return apiClient.doRequest<T>(path, options)
}

interface GatewayEnvelope<T> {
  data: T
  requestId: string
}

export type GatewayPaginatedEnvelope<T> = GatewaySuccessEnvelope<T[]> & {
  page: {
    page: number
    pageSize: number
    total: number
  }
}

/**
 * Paginated-list request via the singleton API client.
 * @see ApiClientImpl.listRequest
 * @deprecated Use `gatewayPageRequest` instead. This wrapper is kept for backward
 * compatibility with existing API modules during migration.
 */
export function listRequest<T>(path: string, options?: RequestInit): Promise<ListResponse<T>> {
  return apiClient.listRequest<T>(path, options)
}

function getAccessToken(): string | null {
  // Primary: unified apiClient-managed token
  const clientToken = apiClient.getToken()
  if (clientToken) return clientToken

  // Fallback: legacy token storage keys (for migration / different auth flows)
  return (
    window.localStorage.getItem('accessToken') ??
    window.localStorage.getItem('qa-access-token') ??
    window.localStorage.getItem('auth.accessToken')
  )
}

type RequestBody = BodyInit | Record<string, unknown> | unknown[] | null

export type GatewayPage = GatewayPaginatedEnvelope<unknown>['page']

export type GatewayRequestOptions = Omit<RequestInit, 'body' | 'method'> & {
  body?: RequestBody
  method?: GatewayMethod
  requestId?: string
  token?: string | null
}

type GatewayStreamOptions = GatewayRequestOptions & {
  onEvent: (event: SseEvent) => void
  onError?: (error: ApiError) => void
  onDone?: () => void
}

export type SseEvent = {
  event: string
  data: string
  id?: string
  retry?: number
}

type MockHandler = (request: Request) => Response | Promise<Response>

type MockRoute = {
  method: GatewayMethod
  path: GatewayPath
  handler: MockHandler
}

let accessTokenProvider: (() => string | null | undefined) | undefined
let requestIdProvider: (() => string | undefined) | undefined
let mockRoutes: MockRoute[] = []

export const apiClient = {
  get baseUrl() {
    return getGatewayBaseUrl()
  },
  setAccessTokenProvider(provider: typeof accessTokenProvider) {
    accessTokenProvider = provider
  },
  setRequestIdProvider(provider: typeof requestIdProvider) {
    requestIdProvider = provider
  },
  setMockRoutes(routes: readonly MockRoute[]) {
    mockRoutes = routes.map((route) => {
      assertActiveGatewayPath(route.path)
      return route
    })
  },
  clearMockRoutes() {
    mockRoutes = []
  },
}

function getGatewayBaseUrl(): string {
  const configured = import.meta.env?.VITE_API_BASE_URL as string | undefined
  return stripTrailingSlash(configured || DEFAULT_GATEWAY_BASE_URL)
}

function stripTrailingSlash(value: string): string {
  return value.endsWith('/') ? value.slice(0, -1) : value
}

function joinUrl(
  path: string,
  query?: URLSearchParams | Record<string, string | number | boolean | null | undefined>,
): string {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  const url = `${getGatewayBaseUrl()}${normalizedPath}`
  const params = query instanceof URLSearchParams ? query : toSearchParams(query)
  const queryString = params?.toString()
  return queryString ? `${url}?${queryString}` : url
}

function toSearchParams(
  query?: Record<string, string | number | boolean | null | undefined>,
): URLSearchParams | undefined {
  if (!query) return undefined
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(query)) {
    if (value == null) continue
    params.set(key, String(value))
  }
  return params
}

function buildHeaders(options: GatewayRequestOptions, hasJsonBody: boolean): Headers {
  const headers = new Headers(options.headers)
  const token = options.token ?? accessTokenProvider?.()
  const requestId = options.requestId ?? requestIdProvider?.()

  if (hasJsonBody && !headers.has('Content-Type')) {
    headers.set('Content-Type', JSON_CONTENT_TYPE)
  }
  if (!headers.has('Accept')) {
    headers.set('Accept', JSON_CONTENT_TYPE)
  }
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }
  if (requestId) {
    headers.set('X-Request-Id', requestId)
  }

  return headers
}

function prepareBody(body: RequestBody | undefined): { body?: BodyInit; hasJsonBody: boolean } {
  if (body == null) return { hasJsonBody: false }
  if (
    body instanceof FormData ||
    body instanceof Blob ||
    body instanceof ArrayBuffer ||
    body instanceof URLSearchParams ||
    typeof body === 'string'
  ) {
    return { body, hasJsonBody: false }
  }
  return { body: JSON.stringify(body), hasJsonBody: true }
}

function isGatewayErrorEnvelope(value: unknown): value is GatewayErrorEnvelope {
  return Boolean(
    value &&
    typeof value === 'object' &&
    'error' in value &&
    (value as { error?: unknown }).error &&
    typeof (value as { error: { message?: unknown } }).error.message === 'string',
  )
}

async function readJsonSafely(response: Response): Promise<unknown> {
  const text = await response.text()
  if (!text) return undefined
  try {
    return JSON.parse(text) as unknown
  } catch {
    return text
  }
}

async function toApiError(response: Response): Promise<ApiError> {
  const body = await readJsonSafely(response)
  if (isGatewayErrorEnvelope(body)) {
    return new ApiError({
      code: body.error.code,
      message: body.error.message,
      status: response.status,
      requestId: body.error.requestId ?? response.headers.get('X-Request-Id') ?? undefined,
      fields: body.error.fields,
    })
  }

  return new ApiError({
    code: response.status ? `http_${response.status}` : 'network_error',
    message: typeof body === 'string' && body ? body : response.statusText || 'Request failed',
    status: response.status,
    requestId: response.headers.get('X-Request-Id') ?? undefined,
  })
}

function assertActiveGatewayPath(path: GatewayPath): void {
  if (!activeGatewayPathSet.has(path)) {
    throw new Error(`Mock path is not an active gateway OpenAPI path: ${path}`)
  }
}

function matchMock(method: GatewayMethod, path: string): MockRoute | undefined {
  if (import.meta.env?.VITE_API_MOCKS !== 'true') return undefined
  return mockRoutes.find((route) => route.method === method && route.path === path)
}

async function fetchGateway(path: string, options: GatewayRequestOptions = {}): Promise<Response> {
  const method = options.method ?? 'GET'
  const mock = matchMock(method, path)
  const { body, hasJsonBody } = prepareBody(options.body)
  const headers = buildHeaders(options, hasJsonBody)
  const request = new Request(joinUrl(path), {
    ...options,
    method,
    headers,
    body,
  })

  if (mock) return mock.handler(request)
  return fetch(request)
}

export async function requestEnvelope<T>(
  path: string,
  options?: GatewayRequestOptions,
): Promise<GatewaySuccessEnvelope<T>> {
  const response = await fetchGateway(path, options)
  if (!response.ok) throw await toApiError(response)
  return (await response.json()) as GatewaySuccessEnvelope<T>
}

export async function requestPaginated<T>(
  path: string,
  options?: GatewayRequestOptions,
): Promise<GatewayPaginatedEnvelope<T>> {
  const response = await fetchGateway(path, options)
  if (!response.ok) throw await toApiError(response)
  return (await response.json()) as GatewayPaginatedEnvelope<T>
}

export async function requestJson<T>(path: string, options?: GatewayRequestOptions): Promise<T> {
  const envelope = await requestEnvelope<T>(path, options)
  return envelope.data
}

export async function requestVoid(path: string, options?: GatewayRequestOptions): Promise<void> {
  const response = await fetchGateway(path, options)
  if (!response.ok) throw await toApiError(response)
}

export async function requestBinary(path: string, options?: GatewayRequestOptions): Promise<Blob> {
  const response = await fetchGateway(path, {
    ...options,
    headers: {
      Accept: 'application/octet-stream',
      ...options?.headers,
    },
  })
  if (!response.ok) throw await toApiError(response)
  return response.blob()
}

export function buildQuery(
  params: Record<string, string | number | boolean | undefined | null>,
): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== '') {
      search.set(key, String(value))
    }
  }

  const query = search.toString()
  return query ? `?${query}` : ''
}

async function readGatewayError(res: Response): Promise<ApiError> {
  const fallbackMessage = `HTTP ${res.status}: ${res.statusText}`

  try {
    const json = (await res.json()) as GatewayErrorEnvelope
    const error = json.error
    return new ApiError(
      String(error?.code ?? res.status),
      error?.message ?? fallbackMessage,
      error?.requestId ?? '',
      error?.fields,
    )
  } catch {
    return new ApiError(String(res.status), fallbackMessage, '')
  }
  return { ...options, headers }
}

export function gatewayRequest<T>(path: string, options?: GatewayRequestOptions): Promise<T> {
  return requestJson<T>(path, withJsonHeaders(options))
}

export async function gatewayPageRequest<T>(
  path: string,
  options?: GatewayRequestOptions,
): Promise<{ items: T[]; page: GatewayPage }> {
  const envelope = await requestPaginated<T>(path, withJsonHeaders(options))
  return { items: envelope.data, page: envelope.page }
}

export function gatewayFileRequest(path: string, options?: GatewayRequestOptions): Promise<Blob> {
  return requestBinary(path, {
    ...options,
    headers: {
      Accept:
        'application/vnd.openxmlformats-officedocument.wordprocessingml.document, application/octet-stream',
      ...options?.headers,
    },
  })
}

export function streamGateway(
  path: string,
  options: GatewayStreamOptions,
): { abort: () => void; signal: AbortSignal } {
  const controller = new AbortController()
  const signal = mergeAbortSignals(controller.signal, options.signal)

  void (async () => {
    try {
      const response = await fetchGateway(path, {
        ...options,
        method: options.method ?? 'POST',
        signal,
        headers: {
          Accept: SSE_CONTENT_TYPE,
          ...options.headers,
        },
      })

      if (!response.ok) throw await toApiError(response)
      const contentType = response.headers.get('Content-Type') ?? ''
      if (!contentType.includes(SSE_CONTENT_TYPE)) {
        throw new ApiError({
          code: 'invalid_stream_response',
          message: 'Expected text/event-stream response',
          status: response.status,
          requestId: response.headers.get('X-Request-Id') ?? undefined,
        })
      }
      if (!response.body) {
        throw new ApiError({
          code: 'empty_stream_response',
          message: 'Response body is not readable',
          status: response.status,
          requestId: response.headers.get('X-Request-Id') ?? undefined,
        })
      }

      await readSseStream(response.body, options.onEvent, signal)
      options.onDone?.()
    } catch (error) {
      if (signal.aborted) return
      options.onError?.(
        error instanceof ApiError
          ? error
          : new ApiError({
              code: 'network_error',
              message: error instanceof Error ? error.message : 'Network error',
              status: 0,
            }),
      )
    }
  })()

  return { abort: () => controller.abort(), signal }
}

function mergeAbortSignals(primary: AbortSignal, secondary?: AbortSignal | null): AbortSignal {
  if (!secondary) return primary
  const controller = new AbortController()
  const abort = (signal: AbortSignal) => controller.abort(signal.reason)

  if (primary.aborted) {
    abort(primary)
    return controller.signal
  }
  if (secondary.aborted) {
    abort(secondary)
    return controller.signal
  }

  primary.addEventListener('abort', () => abort(primary), { once: true })
  secondary.addEventListener('abort', () => abort(secondary), { once: true })
  return controller.signal
}

/**
 * Streaming request — returns raw Response for SSE / event-stream consumption.
 *
 * Attaches auth and request-id headers but does NOT parse the response body.
 * The caller is responsible for reading the stream and handling errors.
 *
 * @example
 *   const res = await gatewayStreamRequest(`/qa-sessions/${id}/messages`, {
 *     method: 'POST',
 *     body: JSON.stringify({ message: 'hello' }),
 *   })
 *   // res.body is a ReadableStream for SSE parsing
 */
export async function gatewayStreamRequest(
  path: string,
  options?: RequestInit,
): Promise<Response> {
  const body = options?.body ?? null
  const res = await fetch(`${apiClient.baseUrl}${path}`, {
    ...options,
    headers: {
      ...gatewayHeaders(body),
      Accept: 'text/event-stream',
      ...options?.headers,
    },
  })

  if (!res.ok) {
    throw await readGatewayError(res)
  }

  return res
}
