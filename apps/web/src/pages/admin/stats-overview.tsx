import { useQuery } from '@tanstack/react-query'
import { BarChart3, Database, FileText, MessageSquare, Timer, Users } from 'lucide-react'

import { getQAMetricsOverview } from '@/api/admin'
import type { QAMetricsOverview } from '@/lib/types'

const statCards = [
  {
    key: 'totalQaCount' as const,
    label: '总问答次数',
    icon: MessageSquare,
    format: (v: number | undefined) => (v ?? 0).toLocaleString(),
  },
  {
    key: 'todayQaCount' as const,
    label: '今日问答',
    icon: BarChart3,
    format: (v: number | undefined) => (v ?? 0).toLocaleString(),
  },
  {
    key: 'avgLatencyMs' as const,
    label: '平均延迟',
    icon: Timer,
    format: (v: number | undefined) => `${Math.round(v ?? 0)} ms`,
  },
  {
    key: 'activeUsersToday' as const,
    label: '今日活跃用户',
    icon: Users,
    format: (v: number | undefined) => (v ?? 0).toLocaleString(),
  },
  {
    key: 'knowledgeBaseCount' as const,
    label: '知识库数量',
    icon: Database,
    format: (v: number | undefined) => (v ?? 0).toLocaleString(),
  },
  {
    key: 'documentCount' as const,
    label: '文档总数',
    icon: FileText,
    format: (v: number | undefined) => (v ?? 0).toLocaleString(),
  },
] as const

function StatCardSkeleton() {
  return (
    <div className="animate-pulse rounded-lg border border-border bg-card p-5">
      <div className="mb-3 h-4 w-20 rounded bg-muted" />
      <div className="h-8 w-24 rounded bg-muted" />
    </div>
  )
}

function StatCard({
  label,
  value,
  icon: Icon,
}: {
  label: string
  value: string
  icon: React.ComponentType<{ className?: string; 'aria-hidden'?: boolean | 'true' }>
}) {
  return (
    <div className="rounded-lg border border-border bg-card p-5 transition-shadow hover:shadow-sm">
      <div className="mb-2 flex items-center gap-2 text-sm text-muted-foreground">
        <Icon aria-hidden="true" className="size-4" />
        <span>{label}</span>
      </div>
      <p className="text-2xl font-bold text-foreground">{value}</p>
    </div>
  )
}

export function StatsOverviewPage() {
  const { data, isLoading, isError, error } = useQuery<QAMetricsOverview>({
    queryKey: ['admin', 'qa-metrics-overview'],
    queryFn: () => getQAMetricsOverview(),
    staleTime: 60_000,
    refetchInterval: 300_000,
  })

  return (
    <div>
      <h3 className="mb-4 text-2xl font-semibold text-foreground">统计概览</h3>
      <p className="mb-6 text-sm text-muted-foreground">
        系统运行统计数据概览，包括问答统计、用户活跃度、知识库规模等。
      </p>

      {/* Error state */}
      {isError && (
        <div className="mb-6 rounded-lg border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive">
          加载统计数据失败: {error instanceof Error ? error.message : '未知错误'}
        </div>
      )}

      {/* Stats grid */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {isLoading
          ? Array.from({ length: 6 }).map((_, i) => <StatCardSkeleton key={i} />)
          : data &&
            statCards.map(({ key, label, icon, format }) => (
              <StatCard
                key={key}
                label={label}
                value={format(data[key] as number | undefined)}
                icon={icon}
              />
            ))}
      </div>

      {/* Empty state (edge case) */}
      {!isLoading && !isError && !data && (
        <div className="rounded-lg border border-dashed border-border p-10 text-center text-sm text-muted-foreground">
          暂无统计数据
        </div>
      )}
    </div>
  )
}
