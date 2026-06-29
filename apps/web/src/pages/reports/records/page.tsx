import { Link } from '@tanstack/react-router'
import { FilePlus2, Search, Trash2 } from 'lucide-react'
import { useState } from 'react'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import type { Report } from '@/features/reports'
import { useDeleteReport, useReportsQuery } from '@/features/reports'

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
  const reportsQuery = useReportsQuery(keyword)
  const deleteMutation = useDeleteReport()
  const isFallback = reportsQuery.isError
  const reports = isFallback
    ? fallbackReports.filter((report) => report.name.includes(keyword))
    : (reportsQuery.data?.items ?? [])

  const handleDelete = () => {
    if (!deleteTarget) return
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
        <Button render={<Link to="/reports/generate" />}>
          <FilePlus2 className="size-4" />
          新建报告
        </Button>
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
        <div className="mb-4 rounded-lg border border-border bg-card px-4 py-3 text-sm text-muted-foreground">
          gateway 暂未联通，当前展示本地报告记录示例。
        </div>
      )}

      <div className="overflow-hidden rounded-lg border border-border bg-card">
        <table className="w-full border-collapse text-sm">
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
              <tr key={report.id} className="border-t border-border hover:bg-muted/30 transition-colors">
                <td className="px-4 py-3 font-medium">{report.name}</td>
                <td className="px-4 py-3 text-muted-foreground">{report.reportType}</td>
                <td className="px-4 py-3 text-muted-foreground">{report.year ?? '-'}</td>
                <td className="px-4 py-3">
                  <span className="rounded-full bg-muted px-2 py-1 text-xs">{report.status}</span>
                </td>
                <td className="px-4 py-3 text-muted-foreground">
                  {formatDate(report.updatedAt ?? report.createdAt)}
                </td>
                <td className="px-4 py-3">
                  {!isFallback && (
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

      <Dialog open={Boolean(deleteTarget)} onOpenChange={(open) => !open && setDeleteTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>确定删除此报告？</DialogTitle>
            <DialogDescription>
              {deleteTarget?.name
                ? `即将删除报告"${deleteTarget.name}"。此操作不可撤销。`
                : '此操作不可撤销。'}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteTarget(null)}>
              取消
            </Button>
            <Button
              variant="destructive"
              onClick={handleDelete}
              disabled={deleteMutation.isPending}
            >
              {deleteMutation.isPending ? '删除中...' : '确认删除'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
