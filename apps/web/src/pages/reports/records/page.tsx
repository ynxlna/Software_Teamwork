import { Link } from '@tanstack/react-router'
import { FilePlus2, Search, Trash2 } from 'lucide-react'
import { useState } from 'react'

import { ConfirmDialog, InlineNotice, StateBlock, TableSkeleton } from '@/components/common'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { Report } from '@/features/reports'
import { useDeleteReport, useReportsQuery } from '@/features/reports'
import { canAccess } from '@/lib/permissions'
import { useAuthStore } from '@/stores/auth-store'

const reportWriteAccess = { any: ['report:write', 'reports:write'] }

const fallbackReports: Report[] = [
  {
    id: 'report-20260628-001',
    name: '2026年迎峰度夏检查报告',
    reportType: 'summer_peak_inspection',
    templateId: 'tpl-local-summer',
    topic: '迎峰度夏设备安全检查',
    specialty: '电气一次',
    businessObject: '主变、厂用电系统、保护装置',
    year: 2026,
    status: 'generated',
    latestJobId: 'job-local-content',
    latestReportFileId: 'file-local-docx',
    createdAt: '2026-06-28T10:00:00Z',
    updatedAt: '2026-06-28T14:28:00Z',
  },
  {
    id: 'report-20260628-002',
    name: '煤库存审计报告',
    reportType: 'coal_inventory_audit',
    templateId: 'tpl-local-coal',
    topic: '燃煤库存盘点与审计分析',
    year: 2026,
    status: 'outline_generated',
    createdAt: '2026-06-28T09:00:00Z',
  },
]

function formatDate(value?: string): string {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

export function ReportRecordsPage() {
  const [keyword, setKeyword] = useState('')
  const [deleteTarget, setDeleteTarget] = useState<Report | null>(null)
  const user = useAuthStore((state) => state.user)
  const reportsQuery = useReportsQuery(keyword)
  const deleteMutation = useDeleteReport()
  const isFallback = reportsQuery.isError
  const canWriteReports = canAccess(user, reportWriteAccess)
  const reports = isFallback
    ? fallbackReports.filter((report) => report.name.includes(keyword))
    : (reportsQuery.data?.items ?? [])

  const handleDelete = () => {
    if (!canWriteReports || !deleteTarget) return
    deleteMutation.mutate(deleteTarget.id)
    setDeleteTarget(null)
  }

  return (
    <div className="h-full overflow-auto bg-background p-6">
      <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold">报告记录</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            分页查询 /api/v1/reports，后端保留报告、任务和导出文件审计链路。
          </p>
        </div>
        {canWriteReports && (
          <Button render={<Link to="/reports/generate" />}>
            <FilePlus2 className="size-4" />
            新建报告
          </Button>
        )}
      </div>

      <div className="mb-4 flex max-w-md items-center gap-2">
        <Input
          placeholder="按报告名称搜索"
          value={keyword}
          onChange={(event) => setKeyword(event.target.value)}
        />
        <Button variant="outline" size="icon" aria-label="搜索">
          <Search className="size-4" />
        </Button>
      </div>

      {reportsQuery.isError && (
        <InlineNotice className="mb-4" variant="warning">
          gateway 暂未联通，当前展示本地报告记录示例。
        </InlineNotice>
      )}

      {reportsQuery.isLoading && !isFallback ? (
        <TableSkeleton columns={6} showToolbar={false} />
      ) : reports.length === 0 ? (
        <StateBlock title="暂无报告记录" variant="empty" />
      ) : (
        <div className="overflow-x-auto rounded-lg border border-border bg-card">
          <table className="w-full min-w-[720px] border-collapse text-sm">
            <thead className="bg-muted/60 text-left text-muted-foreground">
              <tr>
                <th className="px-4 py-3 font-medium">报告名称</th>
                <th className="px-4 py-3 font-medium">类型</th>
                <th className="px-4 py-3 font-medium">年份</th>
                <th className="px-4 py-3 font-medium">状态</th>
                <th className="px-4 py-3 font-medium">更新时间</th>
                <th className="w-16 px-4 py-3 font-medium">操作</th>
              </tr>
            </thead>
            <tbody>
              {reports.map((report) => (
                <tr
                  key={report.id}
                  className="border-t border-border transition-colors hover:bg-muted/30"
                >
                  <td className="max-w-72 truncate px-4 py-3 font-medium">{report.name}</td>
                  <td className="px-4 py-3 text-muted-foreground">{report.reportType}</td>
                  <td className="px-4 py-3 text-muted-foreground">{report.year ?? '-'}</td>
                  <td className="px-4 py-3">
                    <span className="rounded-full bg-muted px-2 py-1 text-xs">{report.status}</span>
                  </td>
                  <td className="px-4 py-3 text-muted-foreground">
                    {formatDate(report.updatedAt ?? report.createdAt)}
                  </td>
                  <td className="px-4 py-3">
                    {canWriteReports && !isFallback && (
                      <Button
                        variant="ghost"
                        size="icon-xs"
                        aria-label="删除报告"
                        onClick={() => setDeleteTarget(report)}
                      >
                        <Trash2 className="size-3 text-destructive" />
                      </Button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <ConfirmDialog
        cancelLabel="取消"
        confirmLabel="确认删除"
        description={
          deleteTarget?.name
            ? `即将删除报告"${deleteTarget.name}"。此操作不可撤销。`
            : '此操作不可撤销。'
        }
        onConfirm={handleDelete}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null)
        }}
        open={Boolean(deleteTarget)}
        pending={deleteMutation.isPending}
        pendingLabel="删除中..."
        title="确定删除此报告？"
        variant="destructive"
      />
    </div>
  )
}
