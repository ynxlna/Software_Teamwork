import {
  Check,
  Edit3,
  Loader2,
  MessageSquare,
  Plus,
  Trash2,
  X,
} from 'lucide-react'
import { type KeyboardEvent, useCallback, useEffect, useRef, useState } from 'react'

import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import type { ConversationListItem } from '@/lib/types'
import { cn } from '@/lib/utils'

interface ChatSidebarProps {
  sessions: ConversationListItem[]
  activeId: string
  isLoading: boolean
  onSelect: (id: string) => void
  onCreate: () => void
  onDelete: (id: string) => void
  onRename: (id: string, title: string) => void
}

export default function ChatSidebar({
  sessions,
  activeId,
  isLoading,
  onSelect,
  onCreate,
  onDelete,
  onRename,
}: ChatSidebarProps) {
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editTitle, setEditTitle] = useState('')
  const editInputRef = useRef<HTMLInputElement>(null)

  // Focus and select the inline edit input when entering edit mode
  useEffect(() => {
    if (editingId) {
      editInputRef.current?.focus()
      editInputRef.current?.select()
    }
  }, [editingId])

  // ── Edit helpers ──

  const startEdit = useCallback((id: string, title: string) => {
    setEditingId(id)
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

  // ── Delete with confirmation ──

  const handleDelete = useCallback(
    (id: string) => {
      if (window.confirm('确定删除该会话？')) {
        onDelete(id)
      }
    },
    [onDelete],
  )

  // ── Render ──

  return (
    <aside className="flex w-72 shrink-0 flex-col border-r border-border bg-card">
      {/* ── Header ── */}
      <div className="flex flex-col gap-2 border-b border-border p-4">
        <h2 className="text-lg font-semibold text-foreground">对话历史</h2>
        <Button onClick={onCreate} className="w-full">
          <Plus className="size-4" />
          新建对话
        </Button>
      </div>

      {/* ── Session list ── */}
      <ScrollArea className="flex-1">
        <div className="flex flex-col gap-1 p-2">
          {/* Loading state */}
          {isLoading && sessions.length === 0 && (
            <div className="flex flex-col items-center gap-2 py-8">
              <Loader2
                className="size-5 animate-spin text-muted-foreground"
                aria-label="加载中"
              />
              <span className="text-xs text-muted-foreground">加载会话列表…</span>
            </div>
          )}

          {/* Empty state (after loading) */}
          {!isLoading && sessions.length === 0 && (
            <p className="px-4 py-8 text-center text-sm text-muted-foreground">
              暂无对话记录
            </p>
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
                  'group relative flex w-full flex-col items-start gap-0.5 rounded-md px-3 py-2.5 text-left transition-colors hover:bg-muted',
                  isActive && 'bg-accent text-accent-foreground',
                )}
                onClick={() => onSelect(sess.id)}
                onDoubleClick={() => startEdit(sess.id, sess.title)}
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
                      {sess.title}
                    </span>
                    <span className="text-xs text-muted-foreground">
                      <MessageSquare className="mr-1 inline-block size-3" />
                      {sess.message_count} 条消息
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
                          startEdit(sess.id, sess.title)
                        }}
                        tabIndex={0}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter' || e.key === ' ') {
                            e.stopPropagation()
                            startEdit(sess.id, sess.title)
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
                          handleDelete(sess.id)
                        }}
                        tabIndex={0}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter' || e.key === ' ') {
                            e.stopPropagation()
                            handleDelete(sess.id)
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
    </aside>
  )
}
