import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { ApiError } from './client'
import { apiClient } from './client'
import {
  getDocumentContent,
  listChunks,
  listDocuments,
  listKnowledgeBases,
  runKnowledgeQuery,
  uploadDocument,
} from './knowledge'

function jsonResponse(body: unknown, init?: ResponseInit) {
  return new Response(JSON.stringify(body), {
    headers: { 'Content-Type': 'application/json', ...init?.headers },
    status: init?.status ?? 200,
    statusText: init?.statusText,
  })
}

function pageResponse(data: unknown[], pageSize = 20) {
  return jsonResponse({
    data,
    page: { page: 1, pageSize, total: data.length },
    requestId: 'req-page',
  })
}

describe('knowledge gateway API', () => {
  beforeEach(() => {
    vi.stubEnv('VITE_API_BASE_URL', 'http://gateway.test/api/v1')
    apiClient.setToken('token-knowledge')
  })

  it('lists knowledge bases and documents through Gateway paginated envelopes', async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(pageResponse([{ id: 'kb-1', name: '运行规程' }]))
      .mockResolvedValueOnce(pageResponse([{ id: 'doc-1', name: 'guide.pdf' }]))
    vi.stubGlobal('fetch', fetchMock)

    await expect(listKnowledgeBases({ page: 1, pageSize: 50 })).resolves.toMatchObject({
      items: [{ id: 'kb-1', name: '运行规程' }],
      page: { page: 1, pageSize: 20, total: 1 },
    })
    await expect(
      listDocuments('kb/unsafe id', { page: 2, pageSize: 10, status: 'ready' }),
    ).resolves.toMatchObject({
      items: [{ id: 'doc-1', name: 'guide.pdf' }],
    })

    const kbRequest = fetchMock.mock.calls[0]?.[0]
    const docRequest = fetchMock.mock.calls[1]?.[0]
    expect(kbRequest).toBeInstanceOf(Request)
    expect(docRequest).toBeInstanceOf(Request)
    if (!(kbRequest instanceof Request) || !(docRequest instanceof Request)) {
      throw new Error('expected Request instances')
    }
    expect(kbRequest.url).toBe('http://gateway.test/api/v1/knowledge-bases?page=1&pageSize=50')
    expect(docRequest.url).toBe(
      'http://gateway.test/api/v1/knowledge-bases/kb%2Funsafe%20id/documents?page=2&pageSize=10&status=ready',
    )
    expect(kbRequest.headers.get('Authorization')).toBe('Bearer token-knowledge')
  })

  it('uploads documents as multipart form data without forcing JSON content type', async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      jsonResponse(
        {
          data: {
            chunkCount: 0,
            createdAt: '2026-07-01T00:00:00Z',
            id: 'doc-1',
            knowledgeBaseId: 'kb-1',
            name: 'guide.txt',
            status: 'uploaded',
          },
          requestId: 'req-upload',
        },
        { status: 201 },
      ),
    )
    vi.stubGlobal('fetch', fetchMock)

    await expect(
      uploadDocument('kb-1', new File(['hello'], 'guide.txt', { type: 'text/plain' }), [
        '安全',
        '规程',
      ]),
    ).resolves.toMatchObject({ id: 'doc-1', status: 'uploaded' })

    const request = fetchMock.mock.calls[0]?.[0]
    expect(request).toBeInstanceOf(Request)
    if (!(request instanceof Request)) throw new Error('expected Request')
    expect(request.method).toBe('POST')
    expect(request.url).toBe('http://gateway.test/api/v1/knowledge-bases/kb-1/documents')
    expect(request.headers.get('Accept')).toBe('application/json')
    expect(request.headers.get('Content-Type')).toContain('multipart/form-data')
    const body = await request.text()
    expect(body).toContain('name="file"')
    expect(body).toContain('name="tags"\r\n\r\n安全')
    expect(body).toContain('name="tags"\r\n\r\n规程')
  })

  it('reads chunks and original content from active Gateway document routes', async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(pageResponse([{ chunkIndex: 0, content: 'chunk', id: 'chunk-1' }], 50))
      .mockResolvedValueOnce(
        new Response('raw-content', {
          headers: { 'Content-Type': 'application/octet-stream' },
          status: 200,
        }),
      )
    vi.stubGlobal('fetch', fetchMock)

    await expect(listChunks('doc-1', { page: 1, pageSize: 50 })).resolves.toMatchObject({
      items: [{ chunkIndex: 0, content: 'chunk', id: 'chunk-1' }],
    })
    await expect((await getDocumentContent('doc-1')).text()).resolves.toBe('raw-content')

    const chunkRequest = fetchMock.mock.calls[0]?.[0]
    const contentRequest = fetchMock.mock.calls[1]?.[0]
    expect(chunkRequest).toBeInstanceOf(Request)
    expect(contentRequest).toBeInstanceOf(Request)
    if (!(chunkRequest instanceof Request) || !(contentRequest instanceof Request)) {
      throw new Error('expected Request instances')
    }
    expect(chunkRequest.url).toBe(
      'http://gateway.test/api/v1/documents/doc-1/chunks?page=1&pageSize=50',
    )
    expect(contentRequest.url).toBe('http://gateway.test/api/v1/documents/doc-1/content')
    expect(contentRequest.headers.get('Accept')).toContain('application/octet-stream')
  })

  it('posts knowledge-queries and preserves Gateway error details on failure', async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        jsonResponse(
          {
            data: {
              id: 'query-1',
              query: '变压器',
              results: [],
              trace: {
                embeddingDimension: 1024,
                embeddingModel: 'bge',
                embeddingProvider: 'ai-gateway',
                hitCount: 0,
                qdrantCollection: 'knowledge',
                rerank: false,
                searchTopK: 10,
                scoreThreshold: 0,
              },
            },
            requestId: 'req-query',
          },
          { status: 201 },
        ),
      )
      .mockResolvedValueOnce(
        jsonResponse(
          {
            error: {
              code: 'dependency_error',
              message: 'knowledge service unavailable',
              requestId: 'req-dependency',
            },
          },
          { status: 502 },
        ),
      )
    vi.stubGlobal('fetch', fetchMock)

    await expect(
      runKnowledgeQuery({
        query: '变压器',
        rerank: false,
        scoreThreshold: 0,
        topK: 10,
      }),
    ).resolves.toMatchObject({ id: 'query-1', query: '变压器', results: [] })

    await expect(
      runKnowledgeQuery({
        query: '变压器',
        rerank: false,
        scoreThreshold: 0,
        topK: 10,
      }),
    ).rejects.toMatchObject({
      code: 'dependency_error',
      message: 'knowledge service unavailable',
      requestId: 'req-dependency',
      status: 502,
    } satisfies Partial<ApiError>)

    const request = fetchMock.mock.calls[0]?.[0]
    expect(request).toBeInstanceOf(Request)
    if (!(request instanceof Request)) throw new Error('expected Request')
    expect(request.method).toBe('POST')
    expect(request.url).toBe('http://gateway.test/api/v1/knowledge-queries')
    expect(await request.json()).toEqual({
      query: '变压器',
      rerank: false,
      scoreThreshold: 0,
      topK: 10,
    })
  })
})
