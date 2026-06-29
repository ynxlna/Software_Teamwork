import { AlertTriangle, ArrowUpRight, Check, ChevronDown, ChevronRight } from 'lucide-react'
import { type ReactNode, useEffect, useRef, useState } from 'react'
import ReactMarkdown from 'react-markdown'

import { Button } from '@/components/ui/button'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import type { QACitation, QAMessage, QAThinkingStep } from '@/lib/types'
import { cn } from '@/lib/utils'

// ══════════════════════════════════════════════════════════════════════════════
// Sub-components
// ══════════════════════════════════════════════════════════════════════════════

/* ── Citation tooltip ── */
function CitationTooltip({ c }: { c: QACitation }) {
  const [open, setOpen] = useState(false)

  // Resolve display fields (id is always present; docId/docName are deprecated aliases)
  const displayId = c.citationNo != null ? `[${c.citationNo}]` : c.id
  const docName = c.documentName ?? c.docName ?? '未知文档'
  const text = c.text ?? c.contentPreview ?? ''
  const score = c.score ?? 0

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        className="inline-flex rounded-sm bg-accent px-2 py-0.5 text-xs text-primary transition-colors hover:bg-primary hover:text-primary-foreground"
        onClick={(e) => {
          e.stopPropagation()
        }}
      >
        {displayId}
      </PopoverTrigger>
      <PopoverContent className="w-72">
        <div className="text-sm font-medium">{docName}</div>
        <div className="mt-1 text-sm italic text-muted-foreground">「{text}」</div>
        <div className="mt-1 text-xs text-muted-foreground">相关度: {Math.round(score * 100)}%</div>
      </PopoverContent>
    </Popover>
  )
}

/* ── Thinking panel ── */
function ThinkPanel({ steps, done }: { steps: QAThinkingStep[]; done: boolean }) {
  const [open, setOpen] = useState(!done)

  useEffect(() => {
    if (done) {
      const t = setTimeout(() => setOpen(false), 3000)
      return () => clearTimeout(t)
    }
    setOpen(true)
  }, [done])

  if (steps.length === 0) return null

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <CollapsibleTrigger className="flex w-full items-center gap-1 py-1 text-sm text-muted-foreground transition-colors hover:text-foreground">
        {open ? (
          <ChevronDown className="size-3 shrink-0" />
        ) : (
          <ChevronRight className="size-3 shrink-0" />
        )}
        <span>思考过程 ({steps.length} 步)</span>
        {done && <Check className="size-3 shrink-0 text-green-500" />}
      </CollapsibleTrigger>
      <CollapsibleContent className="mt-1 space-y-1 rounded-md border border-border/50 bg-muted/50 p-3">
        {steps.map((s, i) => (
          <div key={i} className="flex items-center gap-2 text-sm text-muted-foreground">
            {/* Status dot */}
            <span
              className={cn(
                'size-1.5 shrink-0 rounded-full',
                s.status === 'done' && 'bg-green-500',
                s.status === 'running' && 'bg-primary animate-pulse',
                s.status === 'pending' && 'bg-muted-foreground/40 animate-pulse',
                s.status === 'failed' && 'bg-red-500',
              )}
            />
            <span className="flex-1">{s.label ?? s.type}</span>
            {s.status === 'done' && <Check className="size-3 shrink-0 text-green-500" />}
            {s.status === 'running' && (
              <span className="animate-pulse text-xs text-primary">...</span>
            )}
            {s.status === 'failed' && <span className="text-xs text-red-500">失败</span>}
          </div>
        ))}
      </CollapsibleContent>
    </Collapsible>
  )
}

/* ── Markdown content ── */
const markdownComponents = {
  h1: ({ children, ...rest }: { children?: ReactNode } & Record<string, unknown>) => (
    <h1 className="mb-4 mt-6 text-xl font-bold text-foreground" {...rest}>
      {children}
    </h1>
  ),
  h2: ({ children, ...rest }: { children?: ReactNode } & Record<string, unknown>) => (
    <h2 className="mb-3 mt-5 text-lg font-semibold text-foreground" {...rest}>
      {children}
    </h2>
  ),
  h3: ({ children, ...rest }: { children?: ReactNode } & Record<string, unknown>) => (
    <h3 className="mb-2 mt-4 text-base font-semibold text-foreground" {...rest}>
      {children}
    </h3>
  ),
  p: ({ children, ...rest }: { children?: ReactNode } & Record<string, unknown>) => (
    <p className="my-2" {...rest}>
      {children}
    </p>
  ),
  ul: ({ children, ...rest }: { children?: ReactNode } & Record<string, unknown>) => (
    <ul className="my-2 list-disc pl-6" {...rest}>
      {children}
    </ul>
  ),
  ol: ({ children, ...rest }: { children?: ReactNode } & Record<string, unknown>) => (
    <ol className="my-2 list-decimal pl-6" {...rest}>
      {children}
    </ol>
  ),
  li: ({ children, ...rest }: { children?: ReactNode } & Record<string, unknown>) => (
    <li className="my-1" {...rest}>
      {children}
    </li>
  ),
  strong: ({ children, ...rest }: { children?: ReactNode } & Record<string, unknown>) => (
    <strong className="font-semibold text-foreground" {...rest}>
      {children}
    </strong>
  ),
  code: ({
    className: cls,
    children,
    ...rest
  }: { className?: string; children?: ReactNode } & Record<string, unknown>) => {
    const isInline = !cls?.includes('language-')
    if (isInline) {
      return (
        <code className="rounded bg-muted px-1.5 py-0.5 text-xs font-mono" {...rest}>
          {children}
        </code>
      )
    }
    return (
      <code className={cls} {...rest}>
        {children}
      </code>
    )
  },
  pre: ({ children, ...rest }: { children?: ReactNode } & Record<string, unknown>) => (
    <pre className="my-2 overflow-x-auto rounded-md bg-zinc-950 p-4 text-sm text-zinc-50" {...rest}>
      {children}
    </pre>
  ),
}

/* ── Status label for assistant messages ── */
function StatusLabel({ status }: { status: QAMessage['status'] }) {
  if (!status || status === 'completed') return null
  if (status === 'streaming') return null
  if (status === 'stopped' || status === 'cancelled') {
    return (
      <span className="ml-2 text-xs text-muted-foreground" aria-label="回复已停止">
        已停止
      </span>
    )
  }
  if (status === 'failed') {
    return (
      <span className="ml-2 text-xs text-destructive" aria-label="发送失败">
        发送失败
      </span>
    )
  }
  return null
}

/* ── Single message bubble ── */
function MessageBubble({ msg, isStreaming }: { msg: QAMessage; isStreaming: boolean }) {
  const isUser = msg.role === 'user'
  const hasThinking = msg.thinking && msg.thinking.length > 0
  const hasCitations = msg.citations && msg.citations.length > 0

  // Determine effective streaming state
  const effectiveStreaming = msg.status === 'streaming' || (!msg.status && isStreaming)

  // Determine thinking done state
  const thinkingDone =
    msg.status === 'completed' ||
    msg.status === 'stopped' ||
    msg.status === 'cancelled' ||
    msg.status === 'failed' ||
    (!msg.status && !isStreaming)

  return (
    <div
      className={cn('flex max-w-[85%] gap-2', isUser ? 'flex-row-reverse self-end' : 'self-start')}
    >
      {/* Avatar */}
      {isUser ? (
        <div className="flex size-8 shrink-0 items-center justify-center rounded-md bg-primary/20 text-xs font-bold text-primary">
          我
        </div>
      ) : (
        <div className="flex size-8 shrink-0 items-center justify-center rounded-md bg-accent text-xs font-bold text-accent-foreground">
          AI
        </div>
      )}

      {/* Bubble */}
      <div
        className={cn(
          'min-w-0 px-4 py-3',
          isUser
            ? 'rounded-lg rounded-br-sm bg-primary text-primary-foreground'
            : 'rounded-lg rounded-bl-sm border border-border bg-muted',
        )}
      >
        {/* Thinking steps (assistant only) */}
        {hasThinking && (
          <div className="mb-2">
            <ThinkPanel steps={msg.thinking!} done={thinkingDone} />
          </div>
        )}

        {/* Message content */}
        <div className="leading-relaxed">
          {isUser ? (
            <p className="whitespace-pre-wrap">{msg.content}</p>
          ) : msg.content ? (
            <span>
              {/* @ts-expect-error react-markdown Components type mismatch with React 19 */}
              <ReactMarkdown components={markdownComponents}>{msg.content}</ReactMarkdown>
              <StatusLabel status={msg.status} />
            </span>
          ) : effectiveStreaming ? (
            <span>
              <span className="animate-pulse text-primary" aria-label="AI 正在回复中">
                ▊
              </span>
              <StatusLabel status={msg.status} />
            </span>
          ) : msg.status === 'stopped' || msg.status === 'cancelled' || msg.status === 'failed' ? (
            <span>
              <span className="italic text-muted-foreground">（无内容）</span>
              <StatusLabel status={msg.status} />
            </span>
          ) : (
            <span className="italic text-muted-foreground">（无内容）</span>
          )}
        </div>

        {/* Citations (assistant only) */}
        {hasCitations && (
          <div className="mt-4 border-t border-border/50 pt-2">
            <p className="mb-1 text-xs font-semibold text-muted-foreground">引用来源</p>
            <div className="flex flex-wrap gap-1">
              {msg.citations!.map((c) => (
                <CitationTooltip key={c.id} c={c} />
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

/* ── Error banner ── */
function ErrorBanner({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div className="mx-10 flex items-center gap-3 rounded-md border border-red-200 bg-red-50 px-4 py-3 dark:border-red-800 dark:bg-red-950">
      <AlertTriangle className="size-4 shrink-0 text-red-500" aria-hidden="true" />
      <span className="flex-1 text-sm text-red-700 dark:text-red-300">{message}</span>
      <Button variant="destructive" size="sm" onClick={onRetry}>
        重试
      </Button>
    </div>
  )
}

// ══════════════════════════════════════════════════════════════════════════════
// Main component
// ══════════════════════════════════════════════════════════════════════════════

type ChatMessagesProps = {
  messages: QAMessage[]
  streaming: boolean
  error: string | null
  suggestedPrompts: string[]
  onSuggestedClick: (prompt: string) => void
  onRetry: () => void
}

export default function ChatMessages({
  messages,
  streaming,
  error,
  suggestedPrompts,
  onSuggestedClick,
  onRetry,
}: ChatMessagesProps) {
  const bottomRef = useRef<HTMLDivElement>(null)

  // Auto-scroll to bottom when messages or streaming updates
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, streaming])

  const isEmpty = messages.length === 0

  return (
    <div className="flex flex-1 flex-col gap-6 overflow-y-auto p-6">
      {/* ── Welcome / empty state ── */}
      {isEmpty && (
        <div className="flex flex-1 flex-col items-center justify-center gap-4 text-muted-foreground">
          <h2 className="text-3xl font-bold text-foreground">智能问答系统</h2>
          <p className="text-lg">基于 RAG 的电力行业知识问答助手</p>

          <div className="mt-4 flex w-full max-w-[420px] flex-col gap-2">
            {suggestedPrompts.map((p, i) => (
              <button
                key={i}
                type="button"
                className="w-full rounded-md border border-primary/30 bg-primary/5 px-4 py-3 text-left text-sm text-primary transition-all hover:bg-primary/10 hover:border-primary/50"
                onClick={() => onSuggestedClick(p)}
              >
                <ArrowUpRight className="mr-1 inline-block size-3.5 shrink-0" />
                {p}
              </button>
            ))}
          </div>
        </div>
      )}

      {/* ── Message list ── */}
      {messages.map((msg, i) => {
        const isLast = i === messages.length - 1
        const isStreamingAsst = isLast && msg.role === 'assistant' && streaming
        return (
          <div key={msg.id} className="message-enter">
            <MessageBubble msg={msg} isStreaming={isStreamingAsst} />
          </div>
        )
      })}

      {/* ── Error ── */}
      {error && <ErrorBanner message={error} onRetry={onRetry} />}

      {/* ── Scroll anchor ── */}
      <div ref={bottomRef} />
    </div>
  )
}
