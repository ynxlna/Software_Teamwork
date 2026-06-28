import { Settings } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

import { streamChat } from '@/api/chat'
import { ChatInput, ChatMessages, ChatSidebar } from '@/components/chat'
import {
  useCreateSession,
  useDeleteSession,
  useRenameSession,
  useSession,
  useSessions,
} from '@/features/qa'
import type {
  Citation,
  Conversation,
  ConversationListItem,
  Message,
  ThinkingStep,
} from '@/lib/types'
import { useChatStore } from '@/stores/chat-store'

// ══════════════════════════════════════════════════════════════════════════════
// Helpers
// ══════════════════════════════════════════════════════════════════════════════

function nextId(): string {
  return Date.now().toString(36) + Math.random().toString(36).slice(2)
}

function toSessionListItem(s: Conversation): ConversationListItem {
  const last = s.messages[s.messages.length - 1]
  return {
    id: s.id,
    title: s.title,
    message_count: s.messages.length,
    last_message_preview: last ? last.content.slice(0, 50) : '',
    created_at: s.created_at,
    updated_at: s.updated_at,
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

  // ── React Query: active session detail ──
  const { data: sessionDetail, isError: sessionDetailError } = useSession(
    activeId ?? '',
  )

  // ── Local input text ──
  const [inputText, setInputText] = useState('')

  // ── Mutations ──
  const createSessionMut = useCreateSession()
  const deleteSessionMut = useDeleteSession()
  const renameSessionMut = useRenameSession()

  // ── SSE cleanup ref ──
  const abortRef = useRef<(() => void) | null>(null)

  // ══════════════════════════════════════════════════════════════════════════
  // Refresh recovery: sync server list into store
  // ══════════════════════════════════════════════════════════════════════════

  useEffect(() => {
    if (sessionsData?.items) {
      const currentSessions = useChatStore.getState().sessions
      const merged: Conversation[] = sessionsData.items.map((item) => {
        const existing = currentSessions.find((s) => s.id === item.id)
        if (existing) {
          // Preserve in-memory messages; update metadata from server
          return { ...existing, title: item.title, updated_at: item.updated_at }
        }
        return {
          id: item.id,
          title: item.title,
          messages: [] as Message[],
          created_at: item.created_at,
          updated_at: item.updated_at,
        }
      })
      setSessions(merged)
    }
  }, [sessionsData, setSessions])

  // ══════════════════════════════════════════════════════════════════════════
  // Fetch active session messages from server (for refresh recovery)
  // ══════════════════════════════════════════════════════════════════════════

  useEffect(() => {
    if (sessionDetail && activeId) {
      const current = useChatStore
        .getState()
        .sessions.find((s) => s.id === activeId)
      // Only overwrite if local messages are empty (don't clobber streaming data)
      if (current && current.messages.length === 0 && sessionDetail.messages.length > 0) {
        updateSessionMessages(activeId, sessionDetail.messages)
      }
    }
  }, [sessionDetail, activeId, updateSessionMessages])

  // ══════════════════════════════════════════════════════════════════════════
  // Surface session detail fetch error when local messages are empty
  // ══════════════════════════════════════════════════════════════════════════

  useEffect(() => {
    if (sessionDetailError && activeId) {
      const local = useChatStore
        .getState()
        .sessions.find((s) => s.id === activeId)
      if (!local || local.messages.length === 0) {
        setError('加载会话消息失败，请检查网络连接')
      }
    }
  }, [sessionDetailError, activeId, setError])

  // ══════════════════════════════════════════════════════════════════════════
  // Cleanup SSE on unmount
  // ══════════════════════════════════════════════════════════════════════════

  useEffect(() => {
    return () => {
      abortRef.current?.()
    }
  }, [])

  // ══════════════════════════════════════════════════════════════════════════
  // Derive sidebar items
  // ══════════════════════════════════════════════════════════════════════════

  const sidebarItems: ConversationListItem[] = useMemo(
    () => sessions.map(toSessionListItem),
    [sessions],
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
    async (id: string) => {
      try {
        await deleteSessionMut.mutateAsync(id)
        removeSession(id)
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
        setSessions(
          current.map((s) =>
            s.id === sessionId ? { ...s, title: newTitle } : s,
          ),
        )
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
          const title =
            trimmed.slice(0, 30) + (trimmed.length > 30 ? '…' : '')
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
      const userMsg: Message = {
        id: nextId(),
        role: 'user',
        content: trimmed,
        timestamp: new Date().toISOString(),
      }
      const asstMsg: Message = {
        id: nextId(),
        role: 'assistant',
        content: '',
        timestamp: new Date().toISOString(),
        status: 'streaming',
        thinking: [],
        citations: [],
      }

      useChatStore.setState((state) => ({
        sessions: state.sessions.map((s) => {
          if (s.id !== uid) return s
          const isFirst = s.messages.length === 0
          return {
            ...s,
            title: isFirst
              ? trimmed.slice(0, 30) + (trimmed.length > 30 ? '…' : '')
              : s.title,
            messages: [...s.messages, userMsg, asstMsg],
            updated_at: new Date().toISOString(),
          }
        }),
      }))

      setStreaming(true)

      // Accumulators for SSE events
      let content = ''
      const steps: ThinkingStep[] = []
      const cites: Citation[] = []

      /**
       * Patch the last assistant message in the active session.
       * Uses Zustand setState with functional updater for latest state.
       */
      const patchAssistant = (patch: {
        content?: string
        thinking?: ThinkingStep[]
        citations?: Citation[]
        status?: Message['status']
      }) => {
        useChatStore.setState((state) => ({
          sessions: state.sessions.map((s) => {
            if (s.id !== uid) return s
            const msgs = [...s.messages]
            const lastIdx = msgs.length - 1
            const last = msgs[lastIdx]
            if (!last || last.role !== 'assistant') return s
            msgs[lastIdx] = { ...last, ...patch }
            return { ...s, messages: msgs }
          }),
        }))
      }

      // Seq verification helper
      let lastSeq = -1
      const verifySeq = (seq: number): boolean => {
        if (seq <= lastSeq) {
          console.warn(
            `[SSE] Out-of-order event: received seq=${seq}, last=${lastSeq}`,
          )
          return false
        }
        lastSeq = seq
        return true
      }

      // Track whether we've received the first token
      let firstToken = false

      // ③ Initiate SSE stream
      const { abort } = streamChat(
        { conversation_id: uid, message: trimmed },
        {
          onIntentStatus(data) {
            if (!verifySeq(data.seq)) return
            const ex = steps.find((s) => s.type === 'intent')
            if (data.status === 'started' && !ex) {
              steps.push({
                type: 'intent',
                label: data.label,
                status: 'running',
              })
            } else if (data.status === 'done' && ex) {
              ex.status = 'done'
              ex.label = data.label
            }
            patchAssistant({ thinking: [...steps] })
          },
          onThinkingStep(data) {
            if (!verifySeq(data.seq)) return
            const idx = steps.findIndex((s) => s.type === data.step.type)
            if (idx >= 0) {
              steps[idx] = data.step
            } else {
              steps.push(data.step)
            }
            patchAssistant({ thinking: [...steps] })
          },
          onToken(data) {
            if (!verifySeq(data.seq)) return
            if (!firstToken) {
              firstToken = true
              patchAssistant({ status: 'streaming' })
            }
            content += data.text
            patchAssistant({ content })
          },
          onCitation(data) {
            if (!verifySeq(data.seq)) return
            cites.push(data.citation)
            patchAssistant({ citations: [...cites] })
          },
          onDone() {
            setStreaming(false)
            abortRef.current = null
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
        },
      )

      abortRef.current = abort
    },
    [
      addSession,
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
        fetchError={
          sessionsError
            ? '加载会话列表失败，请检查网络连接'
            : null
        }
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
          <a
            href="/admin"
            className="flex size-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            title="管理面板"
            aria-label="管理面板"
          >
            <Settings className="size-5" />
          </a>
        </header>

        {/* Messages area */}
        <ChatMessages
          messages={activeSession?.messages ?? []}
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
