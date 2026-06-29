import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

import { replayEvents, sendMessage as _sendMessageApi, streamChat } from '@/api/chat'
import { ChatInput, ChatMessages, ChatSidebar } from '@/components/chat'
import {
  useCreateSession,
  useDeleteSession,
  useRenameSession,
  useSessionMessages,
  useSessions,
} from '@/features/qa'
import type {
  QACitation,
  QAMessage,
  QASession,
  QASessionListItem,
  QAThinkingStep,
} from '@/lib/types'
import { useChatStore } from '@/stores/chat-store'

// ══════════════════════════════════════════════════════════════════════════════
// Helpers
// ══════════════════════════════════════════════════════════════════════════════

function nextId(): string {
  return Date.now().toString(36) + Math.random().toString(36).slice(2)
}

function toSessionListItem(s: QASession, messages: QAMessage[]): QASessionListItem {
  const last = messages[messages.length - 1]
  return {
    id: s.id,
    title: s.title,
    status: s.status,
    messageCount: messages.length,
    lastMessagePreview: last ? last.content.slice(0, 50) : '',
    createdAt: s.createdAt,
    updatedAt: s.updatedAt,
  }
}

const SUGGESTED_PROMPTS = [
  '变压器巡检有哪些要点？',
  '如何判断变压器油是否需要更换？',
  '电力安全工作规程中关于停电操作的规定是什么？',
]

// ══════════════════════════════════════════════════════════════════════════════
// Component
// ══════════════════════════════════════════════════════════════════════════════

export function ChatPage() {
  // ── React Query: sessions list ──
  const {
    data: sessionsData,
    isLoading: sessionsLoading,
    isError: sessionsError,
    refetch: refetchSessions,
  } = useSessions()

  // ── Zustand store ──
  const sessions = useChatStore((s) => s.sessions)
  const setSessions = useChatStore((s) => s.setSessions)
  const activeId = useChatStore((s) => s.activeId)
  const setActiveId = useChatStore((s) => s.setActiveId)
  const streaming = useChatStore((s) => s.streaming)
  const setStreaming = useChatStore((s) => s.setStreaming)
  const error = useChatStore((s) => s.error)
  const setError = useChatStore((s) => s.setError)
  const lastFailedMsg = useChatStore((s) => s.lastFailedMsg)
  const setLastFailedMsg = useChatStore((s) => s.setLastFailedMsg)
  const clearError = useChatStore((s) => s.clearError)
  const addSession = useChatStore((s) => s.addSession)
  const removeSession = useChatStore((s) => s.removeSession)
  const updateSessionMessages = useChatStore((s) => s.updateSessionMessages)
  const appendSessionMessages = useChatStore((s) => s.appendSessionMessages)
  const messagesBySession = useChatStore((s) => s.messagesBySession)

  // ── React Query: messages for active session (loaded separately from QASession) ──
  const { data: serverMessages, isError: messagesError } = useSessionMessages(activeId ?? '')

  // ── Local input text ──
  const [inputText, setInputText] = useState('')

  // ── Mutations ──
  const createSessionMut = useCreateSession()
  const deleteSessionMut = useDeleteSession()
  const renameSessionMut = useRenameSession()

  // ── SSE cleanup ref ──
  const abortRef = useRef<(() => void) | null>(null)

  // ── Event replay: track current responseRunId for reconnect recovery ──
  const responseRunIdRef = useRef<string | null>(null)

  // ══════════════════════════════════════════════════════════════════════════
  // Refresh recovery: sync server list into store
  // ══════════════════════════════════════════════════════════════════════════

  useEffect(() => {
    if (sessionsData?.items) {
      const currentSessions = useChatStore.getState().sessions
      const merged: QASession[] = sessionsData.items.map((item) => {
        const existing = currentSessions.find((s) => s.id === item.id)
        if (existing) {
          // Preserve existing metadata; update title/status/updatedAt from server
          return { ...existing, title: item.title, status: item.status, updatedAt: item.updatedAt }
        }
        return {
          id: item.id,
          title: item.title,
          status: item.status,
          messageCount: item.messageCount,
          lastMessagePreview: item.lastMessagePreview,
          createdAt: item.createdAt,
          updatedAt: item.updatedAt,
        }
      })
      setSessions(merged)
    }
  }, [sessionsData, setSessions])

  // ══════════════════════════════════════════════════════════════════════════
  // Fetch active session messages from server (for refresh recovery)
  // ══════════════════════════════════════════════════════════════════════════

  useEffect(() => {
    if (serverMessages?.items && activeId) {
      const current = useChatStore.getState().messagesBySession[activeId]
      // Only overwrite if local messages are empty (don't clobber streaming data)
      if (!current || current.length === 0) {
        if (serverMessages.items.length > 0) {
          updateSessionMessages(activeId, serverMessages.items)
        }
      }
    }
  }, [serverMessages, activeId, updateSessionMessages])

  // ══════════════════════════════════════════════════════════════════════════
  // Surface messages fetch error when local messages are empty
  // ══════════════════════════════════════════════════════════════════════════

  useEffect(() => {
    if (messagesError && activeId) {
      const local = useChatStore.getState().messagesBySession[activeId]
      if (!local || local.length === 0) {
        setError('加载会话消息失败，请检查网络连接')
      }
    }
  }, [messagesError, activeId, setError])

  // ══════════════════════════════════════════════════════════════════════════
  // Cleanup SSE on unmount
  // ══════════════════════════════════════════════════════════════════════════

  useEffect(() => {
    return () => {
      abortRef.current?.()
    }
  }, [])

  // ══════════════════════════════════════════════════════════════════════════
  // Derive sidebar items (merge sessions + messages for display)
  // ══════════════════════════════════════════════════════════════════════════

  const sidebarItems: QASessionListItem[] = useMemo(
    () =>
      sessions.map((s) => {
        const msgs = messagesBySession[s.id] ?? []
        return toSessionListItem(s, msgs)
      }),
    [sessions, messagesBySession],
  )

  // ══════════════════════════════════════════════════════════════════════════
  // Create session
  // ══════════════════════════════════════════════════════════════════════════

  const handleCreate = useCallback(async () => {
    try {
      const newSession = await createSessionMut.mutateAsync('新对话')
      addSession(newSession)
      setActiveId(newSession.id)
    } catch {
      setError('创建会话失败，请检查网络连接')
    }
  }, [createSessionMut, addSession, setActiveId, setError])

  // ══════════════════════════════════════════════════════════════════════════
  // Delete session (only remove from UI on API success)
  // ══════════════════════════════════════════════════════════════════════════

  const handleDelete = useCallback(
    async (sessionId: string) => {
      try {
        await deleteSessionMut.mutateAsync(sessionId)
        removeSession(sessionId)
      } catch {
        setError('删除会话失败，请检查网络连接')
      }
    },
    [deleteSessionMut, removeSession, setError],
  )

  // ══════════════════════════════════════════════════════════════════════════
  // Rename session
  // ══════════════════════════════════════════════════════════════════════════

  const handleRename = useCallback(
    async (sessionId: string, newTitle: string) => {
      try {
        await renameSessionMut.mutateAsync({ sessionId, title: newTitle })
        const current = useChatStore.getState().sessions
        setSessions(current.map((s) => (s.id === sessionId ? { ...s, title: newTitle } : s)))
      } catch {
        setError('重命名会话失败')
      }
    },
    [renameSessionMut, setSessions, setError],
  )

  // ══════════════════════════════════════════════════════════════════════════
  // Send message (SSE streaming)
  // ══════════════════════════════════════════════════════════════════════════

  const sendMessage = useCallback(
    async (text: string) => {
      const trimmed = text.trim()
      if (!trimmed || useChatStore.getState().streaming) return

      clearError()

      let targetId: string | null = useChatStore.getState().activeId

      // ① Auto-create session if none active
      if (!targetId) {
        try {
          const title = trimmed.slice(0, 30) + (trimmed.length > 30 ? '…' : '')
          const newSession = await createSessionMut.mutateAsync(title)
          addSession(newSession)
          targetId = newSession.id
          setActiveId(targetId)
        } catch {
          setError('创建会话失败，请检查网络连接')
          return
        }
      }

      const uid: string = targetId

      // ② Push user message + empty assistant message into store
      const userMsg: QAMessage = {
        id: nextId(),
        sessionId: uid,
        role: 'user',
        content: trimmed,
        status: 'completed',
        createdAt: new Date().toISOString(),
      }
      const asstMsg: QAMessage = {
        id: nextId(),
        sessionId: uid,
        role: 'assistant',
        content: '',
        status: 'streaming',
        createdAt: new Date().toISOString(),
        thinking: [],
        citations: [],
      }

      appendSessionMessages(uid, [userMsg, asstMsg])

      // Update session metadata (title for first message)
      useChatStore.setState((state) => ({
        sessions: state.sessions.map((s) => {
          if (s.id !== uid) return s
          const msgs = state.messagesBySession[uid] ?? []
          const isFirst = msgs.length <= 2
          return {
            ...s,
            title: isFirst ? trimmed.slice(0, 30) + (trimmed.length > 30 ? '…' : '') : s.title,
            updatedAt: new Date().toISOString(),
          }
        }),
      }))

      setStreaming(true)

      // Accumulators for SSE events
      let content = ''
      const steps: QAThinkingStep[] = []
      const cites: QACitation[] = []

      /**
       * Patch the last assistant message in the active session.
       * Uses Zustand setState with functional updater for latest state.
       */
      const patchAssistant = (patch: {
        id?: string
        content?: string
        thinking?: QAThinkingStep[]
        citations?: QACitation[]
        status?: QAMessage['status']
      }) => {
        useChatStore.setState((state) => {
          const msgs = [...(state.messagesBySession[uid] ?? [])]
          const lastIdx = msgs.length - 1
          const last = msgs[lastIdx]
          if (!last || last.role !== 'assistant') return state
          msgs[lastIdx] = { ...last, ...patch }
          return {
            messagesBySession: {
              ...state.messagesBySession,
              [uid]: msgs,
            },
          }
        })
      }

      // Seq verification helper
      let lastSeq = -1
      const verifySeq = (seq: number): boolean => {
        if (seq <= lastSeq) {
          console.warn(`[SSE] Out-of-order event: received seq=${seq}, last=${lastSeq}`)
          return false
        }
        lastSeq = seq
        return true
      }

      // Track whether we've received the first token
      let firstToken = false

      // ③ Initiate SSE stream
      const { abort } = streamChat(uid, trimmed, {
        onMessageCreated(data) {
          if (!verifySeq(data.seq)) return
          // Capture the real message id and responseRunId from the server
          const serverMsgId = data.messageId as string | undefined
          if (serverMsgId) {
            patchAssistant({ id: serverMsgId })
          }
          const runId = data.responseRunId as string | undefined
          if (runId) responseRunIdRef.current = runId
        },
        onAgentIterationStarted(data) {
          if (!verifySeq(data.seq)) return
          const iterationNo = data.iterationNo as number | undefined
          const label = iterationNo != null ? `Agent 迭代 ${iterationNo}` : 'Agent 分析中'
          const ex = steps.find((s) => s.type === 'agent_iteration' && s.status === 'running')
          if (!ex) {
            steps.push({
              type: 'agent_iteration',
              label,
              status: 'running',
            })
          }
          patchAssistant({ thinking: [...steps] })
        },
        onReasoningStep(data) {
          if (!verifySeq(data.seq)) return
          const step = (data as Record<string, unknown>).step as QAThinkingStep | undefined
          if (!step) return
          const idx = steps.findIndex((s) => s.type === step.type)
          if (idx >= 0) {
            steps[idx] = step
          } else {
            steps.push(step)
          }
          patchAssistant({ thinking: [...steps] })
        },
        onToolStarted(data) {
          if (!verifySeq(data.seq)) return
          const toolName = (data.toolName as string) ?? '工具调用'
          steps.push({
            type: 'tool_call',
            label: `调用: ${toolName}`,
            status: 'running',
          })
          patchAssistant({ thinking: [...steps] })
        },
        onToolCompleted(data) {
          if (!verifySeq(data.seq)) return
          const toolName = (data.toolName as string) ?? '工具'
          // Mark running tool_call step as done
          const idx = steps.findIndex((s) => s.type === 'tool_call' && s.status === 'running')
          if (idx >= 0) {
            steps[idx] = {
              ...steps[idx],
              status: 'done' as const,
              label: `${toolName} 完成`,
            } as QAThinkingStep
          }
          patchAssistant({ thinking: [...steps] })
        },
        onToolFailed(data) {
          if (!verifySeq(data.seq)) return
          const toolName = (data.toolName as string) ?? '工具'
          const idx = steps.findIndex((s) => s.type === 'tool_call' && s.status === 'running')
          if (idx >= 0) {
            steps[idx] = {
              ...steps[idx],
              status: 'failed' as const,
              label: `${toolName} 失败`,
            } as QAThinkingStep
          }
          patchAssistant({ thinking: [...steps] })
        },
        onAnswerDelta(data) {
          if (!verifySeq(data.seq)) return
          if (!firstToken) {
            firstToken = true
            patchAssistant({ status: 'streaming' })
          }
          content += (data.content as string) ?? ''
          patchAssistant({ content })
        },
        onCitationDelta(data) {
          if (!verifySeq(data.seq)) return
          const citation = (data as Record<string, unknown>).citation as QACitation | undefined
          if (citation) {
            cites.push(citation)
            patchAssistant({ citations: [...cites] })
          }
        },
        onAnswerCompleted(data) {
          setStreaming(false)
          abortRef.current = null
          const runId = data.responseRunId as string | undefined
          if (runId) responseRunIdRef.current = runId
          patchAssistant({
            content,
            thinking: [...steps],
            citations: [...cites],
            status: 'completed',
          })
        },
        onError(sseErr) {
          if (!verifySeq(sseErr.seq)) return
          if (sseErr.fatal) {
            setStreaming(false)
            abortRef.current = null
            setError(sseErr.message || '请求失败')
            setLastFailedMsg(trimmed)
            patchAssistant({
              content,
              thinking: [...steps],
              citations: [...cites],
              status: 'failed',
            })
            abort()
          } else {
            // Non-fatal error: attempt event replay to recover missed events
            console.warn(`[SSE] Non-fatal error: ${sseErr.code} - ${sseErr.message}`)
            const runId = responseRunIdRef.current
            if (runId) {
              replayEvents(uid, runId)
                .then((events) => {
                  console.warn(`[SSE] Replayed ${events.length} events for run ${runId}`)
                  // Future: replay events into the conversation state
                })
                .catch((err) => {
                  console.error('[SSE] Event replay failed:', err)
                })
            }
          }
        },
        onAbort() {
          setStreaming(false)
          abortRef.current = null
          patchAssistant({
            content,
            thinking: [...steps],
            citations: [...cites],
            status: 'stopped',
          })
        },
      })

      abortRef.current = abort
    },
    [
      addSession,
      appendSessionMessages,
      clearError,
      createSessionMut,
      setActiveId,
      setError,
      setLastFailedMsg,
      setStreaming,
    ],
  )

  // ══════════════════════════════════════════════════════════════════════════
  // Retry on error
  // ══════════════════════════════════════════════════════════════════════════

  const handleRetry = useCallback(() => {
    if (lastFailedMsg) {
      const msg = lastFailedMsg
      clearError()
      sendMessage(msg)
    }
  }, [lastFailedMsg, clearError, sendMessage])

  // ══════════════════════════════════════════════════════════════════════════
  // Suggested prompt click
  // ══════════════════════════════════════════════════════════════════════════

  const handleSuggested = useCallback(
    (prompt: string) => {
      setInputText(prompt)
      // Defer send so React commits the input text update first
      setTimeout(() => {
        sendMessage(prompt)
        setInputText('')
      }, 0)
    },
    [sendMessage],
  )

  // ══════════════════════════════════════════════════════════════════════════
  // Active session
  // ══════════════════════════════════════════════════════════════════════════

  const activeSession = sessions.find((s) => s.id === activeId)
  const activeMessages = activeId ? (messagesBySession[activeId] ?? []) : []

  // ══════════════════════════════════════════════════════════════════════════
  // Render
  // ══════════════════════════════════════════════════════════════════════════

  return (
    <div className="flex h-full">
      {/* Left: session sidebar */}
      <ChatSidebar
        sessions={sidebarItems}
        activeId={activeId ?? ''}
        isLoading={sessionsLoading}
        fetchError={sessionsError ? '加载会话列表失败，请检查网络连接' : null}
        onRetryFetch={() => refetchSessions()}
        onSelect={setActiveId}
        onCreate={handleCreate}
        onDelete={handleDelete}
        onRename={handleRename}
      />

      {/* Right: main chat area */}
      <div className="flex min-w-0 flex-1 flex-col">
        {/* Header */}
        <header className="flex shrink-0 items-center justify-between border-b border-border bg-muted/30 px-6 py-3">
          <h1 className="text-lg font-semibold text-foreground">
            {activeSession?.title ?? '智能问答'}
          </h1>
        </header>

        {/* Messages area */}
        <ChatMessages
          messages={activeMessages}
          streaming={streaming}
          error={error}
          suggestedPrompts={SUGGESTED_PROMPTS}
          onSuggestedClick={handleSuggested}
          onRetry={handleRetry}
        />

        {/* Input area */}
        <ChatInput
          onSend={sendMessage}
          disabled={streaming}
          value={inputText}
          onChange={setInputText}
        />
      </div>
    </div>
  )
}
