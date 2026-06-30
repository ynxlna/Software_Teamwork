import {
  AlertTriangle,
  CheckCircle2,
  Inbox,
  Info,
  Loader2,
  type LucideIcon,
  ShieldAlert,
} from 'lucide-react'
import type { ReactNode } from 'react'

import { cn } from '@/lib/utils'

type StateBlockVariant =
  'default' | 'loading' | 'empty' | 'error' | 'forbidden' | 'success' | 'warning'

type StateBlockProps = {
  action?: ReactNode
  children?: ReactNode
  className?: string
  description?: ReactNode
  icon?: LucideIcon
  size?: 'compact' | 'default' | 'full'
  title: ReactNode
  variant?: StateBlockVariant
}

const variantIcon: Record<StateBlockVariant, LucideIcon> = {
  default: Info,
  loading: Loader2,
  empty: Inbox,
  error: AlertTriangle,
  forbidden: ShieldAlert,
  success: CheckCircle2,
  warning: AlertTriangle,
}

const variantClassName: Record<StateBlockVariant, string> = {
  default: 'border-border bg-card text-foreground',
  loading: 'border-border bg-card text-foreground',
  empty: 'border-dashed border-border bg-card text-foreground',
  error: 'border-destructive/40 bg-destructive/10 text-destructive',
  forbidden: 'border-destructive/40 bg-destructive/10 text-destructive',
  success:
    'border-emerald-500/30 bg-emerald-50 text-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-200',
  warning:
    'border-amber-500/30 bg-amber-50 text-amber-900 dark:bg-amber-950/30 dark:text-amber-200',
}

const iconClassName: Record<StateBlockVariant, string> = {
  default: 'text-muted-foreground',
  loading: 'animate-spin text-muted-foreground',
  empty: 'text-muted-foreground/60',
  error: 'text-destructive',
  forbidden: 'text-destructive',
  success: 'text-emerald-600 dark:text-emerald-300',
  warning: 'text-amber-600 dark:text-amber-300',
}

export function StateBlock({
  action,
  children,
  className,
  description,
  icon,
  size = 'default',
  title,
  variant = 'default',
}: StateBlockProps) {
  const Icon = icon ?? variantIcon[variant]

  return (
    <section
      className={cn(
        'flex min-w-0 flex-col items-center justify-center rounded-lg border px-4 text-center',
        variantClassName[variant],
        size === 'compact' && 'py-5',
        size === 'default' && 'py-10',
        size === 'full' && 'min-h-[240px] py-12',
        className,
      )}
    >
      <Icon aria-hidden="true" className={cn('mb-3 size-9 shrink-0', iconClassName[variant])} />
      <h2 className="max-w-full text-sm font-semibold [text-wrap:pretty]">{title}</h2>
      {description && (
        <div className="mt-1 max-w-xl text-sm text-muted-foreground [text-wrap:pretty]">
          {description}
        </div>
      )}
      {children && <div className="mt-3 max-w-xl text-sm text-muted-foreground">{children}</div>}
      {action && (
        <div className="mt-4 flex max-w-full flex-wrap justify-center gap-2">{action}</div>
      )}
    </section>
  )
}
