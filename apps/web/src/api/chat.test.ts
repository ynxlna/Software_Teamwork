import { describe, expect, it, vi } from 'vitest'

import { streamChat } from './chat'

function streamResponse(body: string) {
  return new Response(new TextEncoder().encode(body), {
    headers: { 'Content-Type': 'text/event-stream' },
    status: 200,
  })
}

describe('chat stream API', () => {
  it('normalizes answer.delta text payloads to content', async () => {
    const onAnswerDelta = vi.fn()
    const onAnswerCompleted = vi.fn()
    const onError = vi.fn()
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValue(
        streamResponse(
          [
            'event: answer.delta',
            'data: {"text":"root","index":0}',
            '',
            'event: answer.delta',
            'data: {"content":" cause","index":1}',
            '',
            'event: answer.completed',
            'data: {"responseRunId":"run-1"}',
            '',
            '',
          ].join('\n'),
        ),
      )
    vi.stubGlobal('fetch', fetchMock)

    streamChat('session-1', 'question', {
      onAnswerCompleted,
      onAnswerDelta,
      onError,
    })

    await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1))
    expect(onError).not.toHaveBeenCalled()
    await vi.waitFor(() => expect(onAnswerCompleted).toHaveBeenCalledTimes(1))

    expect(onAnswerDelta).toHaveBeenNthCalledWith(
      1,
      expect.objectContaining({ content: 'root', index: 0, seq: 1, text: 'root' }),
    )
    expect(onAnswerDelta).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({ content: ' cause', index: 1, seq: 2 }),
    )
  })

  it('does not treat payload sequenceNo as the cross-event stream seq', async () => {
    const onMessageCreated = vi.fn()
    const onAnswerDelta = vi.fn()
    const onAnswerCompleted = vi.fn()
    const onError = vi.fn()
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValue(
        streamResponse(
          [
            'event: message.created',
            'data: {"messageId":"assistant-1","sequenceNo":99}',
            '',
            'event: answer.delta',
            'data: {"content":"root","sequenceNo":1}',
            '',
            'event: answer.completed',
            'data: {"responseRunId":"run-1"}',
            '',
            '',
          ].join('\n'),
        ),
      )
    vi.stubGlobal('fetch', fetchMock)

    streamChat('session-1', 'question', {
      onAnswerCompleted,
      onAnswerDelta,
      onError,
      onMessageCreated,
    })

    await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1))
    expect(onError).not.toHaveBeenCalled()
    await vi.waitFor(() => expect(onAnswerCompleted).toHaveBeenCalledTimes(1))

    expect(onMessageCreated).toHaveBeenCalledWith(
      expect.objectContaining({ messageId: 'assistant-1', seq: 1, sequenceNo: 99 }),
    )
    expect(onAnswerDelta).toHaveBeenCalledWith(
      expect.objectContaining({ content: 'root', seq: 2, sequenceNo: 1 }),
    )
  })

  it('uses SSE id as the cross-event stream seq before payload seq fields', async () => {
    const onAnswerDelta = vi.fn()
    const onAnswerCompleted = vi.fn()
    const onError = vi.fn()
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValue(
        streamResponse(
          [
            'event: answer.delta',
            'id: 5',
            'data: {"content":"root","seq":99,"eventSeq":88}',
            '',
            'event: answer.completed',
            'id: 6',
            'data: {"responseRunId":"run-1"}',
            '',
            '',
          ].join('\n'),
        ),
      )
    vi.stubGlobal('fetch', fetchMock)

    streamChat('session-1', 'question', {
      onAnswerCompleted,
      onAnswerDelta,
      onError,
    })

    await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1))
    expect(onError).not.toHaveBeenCalled()
    await vi.waitFor(() => expect(onAnswerCompleted).toHaveBeenCalledTimes(1))

    expect(onAnswerDelta).toHaveBeenCalledWith(
      expect.objectContaining({ content: 'root', eventSeq: 88, seq: 5 }),
    )
    expect(onAnswerCompleted).toHaveBeenCalledWith(
      expect.objectContaining({ responseRunId: 'run-1', seq: 6 }),
    )
  })

  it('uses the dispatched max stream seq for fatal stream errors', async () => {
    const onAnswerDelta = vi.fn()
    const onError = vi.fn()
    let pullCount = 0
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(
        new ReadableStream<Uint8Array>({
          pull(controller) {
            pullCount += 1
            if (pullCount === 1) {
              controller.enqueue(
                new TextEncoder().encode(
                  'event: answer.delta\ndata: {"content":"root","seq":50}\n\n',
                ),
              )
              return
            }
            controller.error(new Error('connection lost'))
          },
        }),
        {
          headers: { 'Content-Type': 'text/event-stream' },
          status: 200,
        },
      ),
    )
    vi.stubGlobal('fetch', fetchMock)

    streamChat('session-1', 'question', {
      onAnswerDelta,
      onError,
    })

    await vi.waitFor(() => expect(onAnswerDelta).toHaveBeenCalledTimes(1))
    await vi.waitFor(() => expect(onError).toHaveBeenCalledTimes(1))

    expect(onAnswerDelta).toHaveBeenCalledWith(
      expect.objectContaining({ content: 'root', seq: 50 }),
    )
    expect(onError).toHaveBeenCalledWith(
      expect.objectContaining({
        code: 'network_error',
        fatal: true,
        message: 'connection lost',
        seq: 51,
      }),
    )
  })
})
