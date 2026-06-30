import { AlertTriangle, CheckCircle2, Info, type LucideIcon } from 'lucide-react'
import type { ReactNode } from 'react'

import { cn } from '@/lib/utils'

type InlineNoticeVariant = 'info' | 'success' | 'warning' | 'error'

type InlineNoticeProps = {
  action?: ReactNode
  children: ReactNode
  className?: string
  icon?: LucideIcon
  iconClassName?: string
  title?: ReactNode
  variant?: InlineNoticeVariant
}

const variantIcon: Record<InlineNoticeVariant, LucideIcon> = {
  info: Info,
  success: CheckCircle2,
  warning: AlertTriangle,
  error: AlertTriangle,
}

const variantClassName: Record<InlineNoticeVariant, string> = {
  info: 'border-border bg-card text-muted-foreground',
  success:
    'border-emerald-500/30 bg-emerald-50 text-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-200',
  warning:
    'border-amber-500/30 bg-amber-50 text-amber-900 dark:bg-amber-950/30 dark:text-amber-200',
  error: 'border-destructive/40 bg-destructive/10 text-destructive',
}

export function InlineNotice({
  action,
  children,
  className,
  icon,
  iconClassName,
  title,
  variant = 'info',
}: InlineNoticeProps) {
  const Icon = icon ?? variantIcon[variant]

  return (
    <div
      role={variant === 'error' || variant === 'warning' ? 'alert' : 'status'}
      className={cn(
        'flex min-w-0 items-start gap-3 rounded-lg border px-4 py-3 text-sm',
        variantClassName[variant],
        className,
      )}
    >
      <Icon aria-hidden="true" className={cn('mt-0.5 size-4 shrink-0', iconClassName)} />
      <div className="min-w-0 flex-1">
        {title && <div className="font-medium text-foreground">{title}</div>}
        <div className="break-words [text-wrap:pretty]">{children}</div>
      </div>
      {action && <div className="shrink-0">{action}</div>}
    </div>
  )
}
