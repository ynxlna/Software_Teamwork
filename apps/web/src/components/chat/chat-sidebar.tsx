import { Check, Edit3, MessageSquare, Plus, Trash2, X } from 'lucide-react'
import { type KeyboardEvent, useCallback, useEffect, useRef, useState } from 'react'

import { ConfirmDialog, StateBlock } from '@/components/common'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import type { QASessionListItem } from '@/lib/types'
import { cn } from '@/lib/utils'

type ChatSidebarProps = {
  sessions: QASessionListItem[]
  activeId: string
  isLoading: boolean
  fetchError: string | null
  onRetryFetch: () => void
  onSelect: (sessionId: string) => void
  onCreate: () => void
  onDelete: (sessionId: string) => void
  onRename: (sessionId: string, title: string) => void
}

export default function ChatSidebar({
  sessions,
  activeId,
  isLoading,
  fetchError,
  onRetryFetch,
  onSelect,
  onCreate,
  onDelete,
  onRename,
}: ChatSidebarProps) {
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editTitle, setEditTitle] = useState('')
  const [deleteTargetId, setDeleteTargetId] = useState<string | null>(null)
  const editInputRef = useRef<HTMLInputElement>(null)

  // Focus and select the inline edit input when entering edit mode
  useEffect(() => {
    if (editingId) {
      editInputRef.current?.focus()
      editInputRef.current?.select()
    }
  }, [editingId])

  // ── Edit helpers ──

  const startEdit = useCallback((sessionId: string, title: string) => {
    setEditingId(sessionId)
    setEditTitle(title)
  }, [])

  const confirmEdit = useCallback(() => {
    const id = editingId
    const title = editTitle.trim()
    if (id && title) {
      onRename(id, title)
    }
    setEditingId(null)
  }, [editingId, editTitle, onRename])

  const cancelEdit = useCallback(() => {
    setEditingId(null)
  }, [])

  const handleEditKeyDown = useCallback(
    (e: KeyboardEvent<HTMLInputElement>) => {
      if (e.key === 'Enter') {
        e.preventDefault()
        confirmEdit()
      } else if (e.key === 'Escape') {
        e.preventDefault()
        cancelEdit()
      }
    },
    [confirmEdit, cancelEdit],
  )

  const deleteTarget = sessions.find((session) => session.id === deleteTargetId)

  // ── Render ──

  return (
    <aside className="flex w-72 shrink-0 flex-col border-r border-border bg-card">
      {/* ── Header ── */}
      <div className="flex flex-col gap-2 border-b border-border p-4">
        <h2 className="text-lg font-semibold text-foreground">对话历史</h2>
        <Button
          onClick={onCreate}
          className="group w-full bg-primary text-primary-foreground transition-all duration-200 hover:bg-primary/90 hover:scale-[1.02] hover:shadow-lg active:scale-[0.98]"
        >
          <Plus className="size-4 transition-transform duration-300 group-hover:rotate-90" />
          新建对话
        </Button>
      </div>

      {/* ── Session list ── */}
      <ScrollArea className="flex-1">
        <div className="flex flex-col gap-1 p-2">
          {/* Fetch error state */}
          {fetchError && !isLoading && (
            <StateBlock
              action={
                <Button variant="outline" size="sm" onClick={onRetryFetch}>
                  重新加载
                </Button>
              }
              className="mx-2"
              description={fetchError}
              size="compact"
              title="会话列表加载失败"
              variant="error"
            />
          )}

          {/* Loading state */}
          {!fetchError && isLoading && sessions.length === 0 && (
            <StateBlock className="mx-2" size="compact" title="加载会话列表..." variant="loading" />
          )}

          {/* Empty state (after loading) */}
          {!fetchError && !isLoading && sessions.length === 0 && (
            <StateBlock className="mx-2" size="compact" title="暂无对话记录" variant="empty" />
          )}

          {/* Session items */}
          {sessions.map((sess) => {
            const isEditing = editingId === sess.id
            const isActive = sess.id === activeId

            return (
              <button
                key={sess.id}
                type="button"
                className={cn(
                  'group relative flex w-full flex-col items-start gap-0.5 rounded-md px-3 py-2.5 text-left transition-colors hover:bg-primary/5',
                  isActive && 'bg-primary/10 text-primary border-l-[3px] border-l-primary',
                )}
                onClick={() => onSelect(sess.id)}
                onDoubleClick={() => startEdit(sess.id, sess.title ?? '')}
              >
                {isEditing ? (
                  /* ── Inline rename ── */
                  <span
                    className="flex w-full items-center gap-1"
                    onClick={(e) => e.stopPropagation()}
                    onKeyDown={(e) => {
                      if (e.key === 'Escape') {
                        e.stopPropagation()
                        cancelEdit()
                      }
                    }}
                  >
                    <input
                      ref={editInputRef}
                      className="min-w-0 flex-1 rounded border border-input bg-background px-1.5 py-0.5 text-sm outline-none focus-visible:ring-1 focus-visible:ring-ring"
                      value={editTitle}
                      onChange={(e) => setEditTitle(e.target.value)}
                      onKeyDown={handleEditKeyDown}
                      onBlur={confirmEdit}
                    />
                    <span
                      className="flex size-4 shrink-0 cursor-pointer items-center justify-center rounded text-muted-foreground hover:text-foreground"
                      role="button"
                      title="确认"
                      onClick={confirmEdit}
                      tabIndex={0}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') confirmEdit()
                      }}
                    >
                      <Check className="size-3" />
                    </span>
                    <span
                      className="flex size-4 shrink-0 cursor-pointer items-center justify-center rounded text-muted-foreground hover:text-foreground"
                      role="button"
                      title="取消"
                      onClick={cancelEdit}
                      tabIndex={0}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') cancelEdit()
                      }}
                    >
                      <X className="size-3" />
                    </span>
                  </span>
                ) : (
                  /* ── Normal display ── */
                  <>
                    <span className="w-full truncate pr-14 text-sm font-medium">
                      {sess.title ?? '新对话'}
                    </span>
                    <span className="text-xs text-muted-foreground">
                      <MessageSquare className="mr-1 inline-block size-3" />
                      {sess.messageCount ?? 0} 条消息
                    </span>

                    {/* Action buttons — visible on row hover */}
                    <span className="absolute right-2 top-1/2 -translate-y-1/2 flex items-center gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
                      {/* Edit (rename) button */}
                      <span
                        className="flex size-5 items-center justify-center rounded text-muted-foreground hover:bg-accent hover:text-foreground"
                        role="button"
                        title="重命名"
                        onClick={(e) => {
                          e.stopPropagation()
                          startEdit(sess.id, sess.title ?? '')
                        }}
                        tabIndex={0}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter' || e.key === ' ') {
                            e.stopPropagation()
                            startEdit(sess.id, sess.title ?? '')
                          }
                        }}
                      >
                        <Edit3 className="size-3" aria-hidden="true" />
                      </span>

                      {/* Delete button */}
                      <span
                        className="flex size-5 items-center justify-center rounded text-muted-foreground hover:bg-destructive hover:text-destructive-foreground"
                        role="button"
                        title="删除对话"
                        onClick={(e) => {
                          e.stopPropagation()
                          setDeleteTargetId(sess.id)
                        }}
                        tabIndex={0}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter' || e.key === ' ') {
                            e.stopPropagation()
                            setDeleteTargetId(sess.id)
                          }
                        }}
                      >
                        <Trash2 className="size-3" aria-hidden="true" />
                      </span>
                    </span>
                  </>
                )}
              </button>
            )
          })}
        </div>
      </ScrollArea>
      <ConfirmDialog
        cancelLabel="取消"
        confirmLabel="确认删除"
        description={
          deleteTarget?.title
            ? `即将删除会话"${deleteTarget.title}"。此操作不可撤销。`
            : '此操作不可撤销。'
        }
        onConfirm={() => {
          if (deleteTargetId) onDelete(deleteTargetId)
          setDeleteTargetId(null)
        }}
        onOpenChange={(open) => {
          if (!open) setDeleteTargetId(null)
        }}
        open={Boolean(deleteTargetId)}
        title="确定删除该会话？"
        variant="destructive"
      />
    </aside>
  )
}
