import type { ReactNode } from 'react'

import { cn } from '@/lib/utils'

type ProgressSummaryTone = 'default' | 'success' | 'warning' | 'error'

type ProgressSummaryProps = {
  action?: ReactNode
  className?: string
  description?: ReactNode
  label: ReactNode
  meta?: ReactNode
  percent: number
  status?: ReactNode
  tone?: ProgressSummaryTone
}

const toneClassName: Record<ProgressSummaryTone, string> = {
  default: 'bg-primary',
  success: 'bg-emerald-500',
  warning: 'bg-amber-500',
  error: 'bg-destructive',
}

export function ProgressSummary({
  action,
  className,
  description,
  label,
  meta,
  percent,
  status,
  tone = 'default',
}: ProgressSummaryProps) {
  const value = Math.max(0, Math.min(100, Math.round(percent)))

  return (
    <div className={cn('rounded-lg border border-border bg-background p-3', className)}>
      <div className="flex min-w-0 items-start justify-between gap-3 text-sm">
        <div className="min-w-0">
          <div className="font-medium text-foreground">{label}</div>
          {description && (
            <div className="mt-0.5 break-words text-xs text-muted-foreground [text-wrap:pretty]">
              {description}
            </div>
          )}
        </div>
        {status && <div className="shrink-0 text-xs text-muted-foreground">{status}</div>}
      </div>
      <div className="mt-3">
        <div className="mb-1 flex justify-between text-xs text-muted-foreground">
          <span>进度</span>
          <span>{value}%</span>
        </div>
        <div className="h-2 overflow-hidden rounded-full bg-muted">
          <div
            className={cn('h-full rounded-full transition-all', toneClassName[tone])}
            style={{ width: `${value}%` }}
          />
        </div>
      </div>
      {(meta || action) && (
        <div className="mt-3 flex min-w-0 flex-wrap items-center justify-between gap-2">
          {meta && <div className="min-w-0 text-xs text-muted-foreground">{meta}</div>}
          {action && <div className="shrink-0">{action}</div>}
        </div>
      )}
    </div>
  )
}
