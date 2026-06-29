import {
  BookOpen,
  ChevronLeft,
  ChevronRight,
  Edit,
  Loader2,
  Plus,
  Search,
  Trash2,
} from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'

import { Badge } from '@/components/ui/badge'
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
import {
  useCreateKnowledgeBase,
  useDeleteKnowledgeBase,
  useKnowledgeBases,
  useUpdateKnowledgeBase,
} from '@/features/knowledge'
import type { KnowledgeBaseSummary } from '@/lib/types'

// ── Constants ──

const PAGE_SIZE = 10

const DOC_TYPE_OPTIONS = ['规程规范', '技术报告论文', '术语条目', '通用文档'] as const

const RETRIEVAL_MODE_OPTIONS = [
  { value: 'semantic', label: '语义检索' },
  { value: 'vector_rerank', label: '向量重排序' },
] as const

const RETRIEVAL_MODE_LABELS: Record<string, string> = {
  semantic: '语义检索',
  vector_rerank: '向量重排序',
}

// ── Types ──

type FormData = {
  name: string
  description: string
  docType: string
  retrievalMode: string
}

type NotificationState = {
  type: 'success' | 'error'
  text: string
}

// ── Defaults ──

const EMPTY_FORM: FormData = {
  name: '',
  description: '',
  docType: '通用文档',
  retrievalMode: 'semantic',
}

// ── Skeleton ──

function KnowledgeManagementSkeleton() {
  return (
    <div className="animate-pulse space-y-4">
      {/* Header skeleton */}
      <div className="flex items-center justify-between">
        <div className="h-7 w-32 rounded bg-muted" />
        <div className="h-8 w-28 rounded bg-muted" />
      </div>

      {/* Search skeleton */}
      <div className="flex gap-2">
        <div className="h-8 flex-1 rounded bg-muted" />
        <div className="h-8 w-36 rounded bg-muted" />
      </div>

      {/* Table skeleton */}
      <div className="rounded-lg border border-border bg-card">
        <div className="border-b border-border px-4 py-3">
          <div className="grid grid-cols-6 gap-4">
            {Array.from({ length: 6 }).map((_, i) => (
              <div key={i} className="h-4 rounded bg-muted" />
            ))}
          </div>
        </div>
        <div className="divide-y divide-border">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="grid grid-cols-6 gap-4 px-4 py-3">
              {Array.from({ length: 6 }).map((_, j) => (
                <div key={j} className="h-4 rounded bg-muted" />
              ))}
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

// ── Main component ──

export function KnowledgeManagement() {
  // ── State ──
  const [keyword, setKeyword] = useState('')
  const [docTypeFilter, setDocTypeFilter] = useState('')
  const [page, setPage] = useState(1)

  const [createOpen, setCreateOpen] = useState(false)
  const [editOpen, setEditOpen] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [editingKb, setEditingKb] = useState<KnowledgeBaseSummary | null>(null)
  const [deletingKb, setDeletingKb] = useState<KnowledgeBaseSummary | null>(null)

  const [form, setForm] = useState<FormData>(EMPTY_FORM)
  const [notification, setNotification] = useState<NotificationState | null>(null)

  // ── Queries & mutations ──

  const { data, isLoading, isError, error, refetch } = useKnowledgeBases(
    page,
    PAGE_SIZE,
    keyword || undefined,
    docTypeFilter || undefined,
  )

  const createMutation = useCreateKnowledgeBase()
  const updateMutation = useUpdateKnowledgeBase()
  const deleteMutation = useDeleteKnowledgeBase()

  const isMutating =
    createMutation.isPending || updateMutation.isPending || deleteMutation.isPending

  // ── Notification auto-dismiss ──

  useEffect(() => {
    if (!notification) return
    const timer = setTimeout(() => setNotification(null), 4000)
    return () => clearTimeout(timer)
  }, [notification])

  // ── Handlers ──

  const updateField = useCallback((field: keyof FormData, value: string) => {
    setForm((prev) => ({ ...prev, [field]: value }))
  }, [])

  const openCreate = useCallback(() => {
    setForm(EMPTY_FORM)
    setCreateOpen(true)
  }, [])

  const openEdit = useCallback((kb: KnowledgeBaseSummary) => {
    setEditingKb(kb)
    setForm({
      name: kb.name,
      description: kb.description ?? '',
      docType: kb.docType ?? '通用文档',
      retrievalMode: kb.retrievalStrategy?.mode ?? 'semantic',
    })
    setEditOpen(true)
  }, [])

  const openDelete = useCallback((kb: KnowledgeBaseSummary) => {
    setDeletingKb(kb)
    setDeleteOpen(true)
  }, [])

  const handleCreate = useCallback(() => {
    createMutation.mutate(
      {
        name: form.name,
        description: form.description,
        docType: form.docType,
        retrievalStrategy: { mode: form.retrievalMode },
      },
      {
        onSuccess: () => {
          setNotification({ type: 'success', text: '知识库创建成功' })
          setCreateOpen(false)
          setPage(1)
        },
        onError: (err: Error) => {
          setNotification({ type: 'error', text: `创建失败: ${err.message}` })
        },
      },
    )
  }, [form, createMutation])

  const handleEdit = useCallback(() => {
    if (!editingKb) return
    updateMutation.mutate(
      {
        id: editingKb.id,
        name: form.name,
        description: form.description,
        docType: form.docType,
        retrievalStrategy: { mode: form.retrievalMode },
      },
      {
        onSuccess: () => {
          setNotification({ type: 'success', text: '知识库更新成功' })
          setEditOpen(false)
          setEditingKb(null)
        },
        onError: (err: Error) => {
          setNotification({ type: 'error', text: `更新失败: ${err.message}` })
        },
      },
    )
  }, [editingKb, form, updateMutation])

  const handleDelete = useCallback(() => {
    if (!deletingKb) return
    deleteMutation.mutate(deletingKb.id, {
      onSuccess: () => {
        setNotification({ type: 'success', text: '知识库已删除' })
        setDeleteOpen(false)
        setDeletingKb(null)
      },
      onError: (err: Error) => {
        setNotification({ type: 'error', text: `删除失败: ${err.message}` })
      },
    })
  }, [deletingKb, deleteMutation])

  const handleSearch = useCallback((value: string) => {
    setKeyword(value)
    setPage(1)
  }, [])

  const handleDocTypeFilter = useCallback((value: string) => {
    setDocTypeFilter(value)
    setPage(1)
  }, [])

  // ── Derived ──

  const totalPages = data ? Math.max(1, Math.ceil(data.page.total / PAGE_SIZE)) : 1
  const showPagination = totalPages > 1
  const isEmpty = !isLoading && !isError && data && data.items.length === 0

  // ── Render ──

  return (
    <div>
      {/* Header */}
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h3 className="text-2xl font-semibold text-foreground">知识库管理</h3>
          <p className="mt-1 text-sm text-muted-foreground">
            管理知识库，支持新建、编辑、删除操作。
          </p>
        </div>
        <Button onClick={openCreate}>
          <Plus aria-hidden="true" className="mr-1 size-4" />
          新建知识库
        </Button>
      </div>

      {/* Notification banner */}
      {notification && (
        <div
          role="alert"
          className={`mb-4 rounded-lg border px-4 py-3 text-sm ${
            notification.type === 'success'
              ? 'border-emerald-500/50 bg-emerald-50 text-emerald-800 dark:border-emerald-400/30 dark:bg-emerald-950 dark:text-emerald-300'
              : 'border-destructive/50 bg-destructive/10 text-destructive'
          }`}
        >
          {notification.text}
        </div>
      )}

      {/* Loading state */}
      {isLoading && <KnowledgeManagementSkeleton />}

      {/* Error state */}
      {isError && !isLoading && (
        <div className="rounded-lg border border-destructive/50 bg-destructive/10 p-6 text-center">
          <p className="mb-3 text-sm text-destructive">
            加载知识库失败: {error instanceof Error ? error.message : '未知错误'}
          </p>
          <Button variant="outline" size="sm" onClick={() => refetch()}>
            <Loader2 aria-hidden="true" className="mr-1.5 size-3.5" />
            重试
          </Button>
        </div>
      )}

      {/* Data area */}
      {!isLoading && !isError && (
        <>
          {/* Search & filter bar */}
          <div className="mb-4 flex gap-2">
            <div className="relative flex-1">
              <Search
                aria-hidden="true"
                className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground"
              />
              <Input
                type="text"
                placeholder="搜索知识库名称或描述..."
                value={keyword}
                onChange={(e) => handleSearch(e.target.value)}
                className="pl-8"
              />
            </div>
            <select
              value={docTypeFilter}
              onChange={(e) => handleDocTypeFilter(e.target.value)}
              className="h-8 rounded-lg border border-input bg-transparent px-2.5 py-1 text-sm text-foreground transition-colors outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 md:text-sm"
            >
              <option value="">全部类型</option>
              {DOC_TYPE_OPTIONS.map((dt) => (
                <option key={dt} value={dt}>
                  {dt}
                </option>
              ))}
            </select>
          </div>

          {/* Filter limitation notice */}
          {(keyword || docTypeFilter) && data?.filteredLocally && (
            <p className="text-xs text-muted-foreground">
              搜索仅过滤当前页，其他页的匹配项不会显示。建议使用准确关键词，或切换到更大页码查看。
            </p>
          )}

          {/* Empty state */}
          {isEmpty && (
            <div className="rounded-lg border border-dashed border-border p-12 text-center">
              <BookOpen
                aria-hidden="true"
                className="mx-auto mb-3 size-10 text-muted-foreground/40"
              />
              <p className="text-sm text-muted-foreground">
                {keyword || docTypeFilter
                  ? '未找到匹配的知识库，请调整搜索条件'
                  : '暂无知识库，点击新建知识库开始'}
              </p>
              {!keyword && !docTypeFilter && (
                <Button variant="outline" size="sm" className="mt-3" onClick={openCreate}>
                  <Plus aria-hidden="true" className="mr-1 size-3.5" />
                  新建知识库
                </Button>
              )}
            </div>
          )}

          {/* Table */}
          {data && data.items.length > 0 && (
            <>
              <div className="overflow-x-auto rounded-lg border border-border bg-card">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border bg-muted/30">
                      <th className="px-4 py-2.5 text-left font-medium text-muted-foreground">
                        名称
                      </th>
                      <th className="hidden px-4 py-2.5 text-left font-medium text-muted-foreground md:table-cell">
                        描述
                      </th>
                      <th className="px-4 py-2.5 text-left font-medium text-muted-foreground">
                        类型
                      </th>
                      <th className="px-4 py-2.5 text-right font-medium text-muted-foreground">
                        文档数
                      </th>
                      <th className="hidden px-4 py-2.5 text-left font-medium text-muted-foreground sm:table-cell">
                        检索策略
                      </th>
                      <th className="hidden px-4 py-2.5 text-left font-medium text-muted-foreground lg:table-cell">
                        创建时间
                      </th>
                      <th className="px-4 py-2.5 text-right font-medium text-muted-foreground">
                        操作
                      </th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border">
                    {data.items.map((kb) => (
                      <tr key={kb.id} className="transition-colors hover:bg-muted/30">
                        <td className="max-w-40 truncate px-4 py-2.5 font-medium text-foreground">
                          {kb.name}
                        </td>
                        <td className="hidden max-w-48 truncate px-4 py-2.5 text-muted-foreground md:table-cell">
                          {kb.description || '-'}
                        </td>
                        <td className="px-4 py-2.5">
                          {kb.docType ? (
                            <Badge variant="secondary">{kb.docType}</Badge>
                          ) : (
                            <span className="text-muted-foreground">-</span>
                          )}
                        </td>
                        <td className="px-4 py-2.5 text-right tabular-nums text-muted-foreground">
                          {kb.documentCount}
                        </td>
                        <td className="hidden px-4 py-2.5 text-muted-foreground sm:table-cell">
                          {kb.retrievalStrategy?.mode
                            ? (RETRIEVAL_MODE_LABELS[kb.retrievalStrategy.mode] ??
                              kb.retrievalStrategy.mode)
                            : '-'}
                        </td>
                        <td className="hidden whitespace-nowrap px-4 py-2.5 text-muted-foreground lg:table-cell">
                          {kb.createdAt ? new Date(kb.createdAt).toLocaleDateString('zh-CN') : '-'}
                        </td>
                        <td className="px-4 py-2.5">
                          <div className="flex items-center justify-end gap-1">
                            <Button
                              variant="ghost"
                              size="icon-sm"
                              onClick={() => openEdit(kb)}
                              aria-label={`编辑 ${kb.name}`}
                            >
                              <Edit aria-hidden="true" className="size-3.5" />
                            </Button>
                            <Button
                              variant="ghost"
                              size="icon-sm"
                              onClick={() => openDelete(kb)}
                              aria-label={`删除 ${kb.name}`}
                              className="text-destructive hover:text-destructive"
                            >
                              <Trash2 aria-hidden="true" className="size-3.5" />
                            </Button>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              {/* Pagination */}
              {showPagination && (
                <div className="mt-4 flex items-center justify-between text-sm text-muted-foreground">
                  <span>
                    共 {data.page.total} 条，第 {page} / {totalPages} 页
                  </span>
                  <div className="flex gap-1">
                    <Button
                      variant="outline"
                      size="icon-sm"
                      disabled={page <= 1}
                      onClick={() => setPage((p) => p - 1)}
                      aria-label="上一页"
                    >
                      <ChevronLeft aria-hidden="true" className="size-3.5" />
                    </Button>
                    <Button
                      variant="outline"
                      size="icon-sm"
                      disabled={page >= totalPages}
                      onClick={() => setPage((p) => p + 1)}
                      aria-label="下一页"
                    >
                      <ChevronRight aria-hidden="true" className="size-3.5" />
                    </Button>
                  </div>
                </div>
              )}
            </>
          )}
        </>
      )}

      {/* ── Create Dialog ── */}
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>新建知识库</DialogTitle>
            <DialogDescription>填写知识库的基本信息。</DialogDescription>
          </DialogHeader>

          <div className="space-y-3">
            {/* Name */}
            <div>
              <label
                htmlFor="kb-create-name"
                className="mb-1 block text-sm font-medium text-foreground"
              >
                名称 <span className="text-destructive">*</span>
              </label>
              <Input
                id="kb-create-name"
                type="text"
                placeholder="知识库名称"
                value={form.name}
                onChange={(e) => updateField('name', e.target.value)}
              />
            </div>

            {/* Description */}
            <div>
              <label
                htmlFor="kb-create-desc"
                className="mb-1 block text-sm font-medium text-foreground"
              >
                描述
              </label>
              <Input
                id="kb-create-desc"
                type="text"
                placeholder="知识库描述（可选）"
                value={form.description}
                onChange={(e) => updateField('description', e.target.value)}
              />
            </div>

            {/* DocType */}
            <div>
              <label
                htmlFor="kb-create-doctype"
                className="mb-1 block text-sm font-medium text-foreground"
              >
                文档类型
              </label>
              <select
                id="kb-create-doctype"
                value={form.docType}
                onChange={(e) => updateField('docType', e.target.value)}
                className="h-8 w-full rounded-lg border border-input bg-transparent px-2.5 py-1 text-sm text-foreground transition-colors outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 md:text-sm"
              >
                {DOC_TYPE_OPTIONS.map((dt) => (
                  <option key={dt} value={dt}>
                    {dt}
                  </option>
                ))}
              </select>
            </div>

            {/* Retrieval Strategy */}
            <div>
              <label
                htmlFor="kb-create-retrieval"
                className="mb-1 block text-sm font-medium text-foreground"
              >
                检索策略
              </label>
              <select
                id="kb-create-retrieval"
                value={form.retrievalMode}
                onChange={(e) => updateField('retrievalMode', e.target.value)}
                className="h-8 w-full rounded-lg border border-input bg-transparent px-2.5 py-1 text-sm text-foreground transition-colors outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 md:text-sm"
              >
                {RETRIEVAL_MODE_OPTIONS.map((opt) => (
                  <option key={opt.value} value={opt.value}>
                    {opt.label}
                  </option>
                ))}
              </select>
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)} disabled={isMutating}>
              取消
            </Button>
            <Button onClick={handleCreate} disabled={!form.name.trim() || isMutating}>
              {createMutation.isPending && (
                <Loader2 aria-hidden="true" className="mr-1.5 size-3.5 animate-spin" />
              )}
              创建
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* ── Edit Dialog ── */}
      <Dialog open={editOpen} onOpenChange={setEditOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>编辑知识库</DialogTitle>
            <DialogDescription>修改 "{editingKb?.name}" 的配置信息。</DialogDescription>
          </DialogHeader>

          <div className="space-y-3">
            {/* Name */}
            <div>
              <label
                htmlFor="kb-edit-name"
                className="mb-1 block text-sm font-medium text-foreground"
              >
                名称 <span className="text-destructive">*</span>
              </label>
              <Input
                id="kb-edit-name"
                type="text"
                placeholder="知识库名称"
                value={form.name}
                onChange={(e) => updateField('name', e.target.value)}
              />
            </div>

            {/* Description */}
            <div>
              <label
                htmlFor="kb-edit-desc"
                className="mb-1 block text-sm font-medium text-foreground"
              >
                描述
              </label>
              <Input
                id="kb-edit-desc"
                type="text"
                placeholder="知识库描述（可选）"
                value={form.description}
                onChange={(e) => updateField('description', e.target.value)}
              />
            </div>

            {/* DocType */}
            <div>
              <label
                htmlFor="kb-edit-doctype"
                className="mb-1 block text-sm font-medium text-foreground"
              >
                文档类型
              </label>
              <select
                id="kb-edit-doctype"
                value={form.docType}
                onChange={(e) => updateField('docType', e.target.value)}
                className="h-8 w-full rounded-lg border border-input bg-transparent px-2.5 py-1 text-sm text-foreground transition-colors outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 md:text-sm"
              >
                {DOC_TYPE_OPTIONS.map((dt) => (
                  <option key={dt} value={dt}>
                    {dt}
                  </option>
                ))}
              </select>
            </div>

            {/* Retrieval Strategy */}
            <div>
              <label
                htmlFor="kb-edit-retrieval"
                className="mb-1 block text-sm font-medium text-foreground"
              >
                检索策略
              </label>
              <select
                id="kb-edit-retrieval"
                value={form.retrievalMode}
                onChange={(e) => updateField('retrievalMode', e.target.value)}
                className="h-8 w-full rounded-lg border border-input bg-transparent px-2.5 py-1 text-sm text-foreground transition-colors outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 md:text-sm"
              >
                {RETRIEVAL_MODE_OPTIONS.map((opt) => (
                  <option key={opt.value} value={opt.value}>
                    {opt.label}
                  </option>
                ))}
              </select>
            </div>
          </div>

          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setEditOpen(false)
                setEditingKb(null)
              }}
              disabled={isMutating}
            >
              取消
            </Button>
            <Button onClick={handleEdit} disabled={!form.name.trim() || isMutating}>
              {updateMutation.isPending && (
                <Loader2 aria-hidden="true" className="mr-1.5 size-3.5 animate-spin" />
              )}
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* ── Delete Confirmation Dialog ── */}
      <Dialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>确认删除</DialogTitle>
            <DialogDescription>
              确定要删除知识库 "{deletingKb?.name}"
              吗？此操作不可撤销，知识库中的所有文档也将被删除。
            </DialogDescription>
          </DialogHeader>

          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setDeleteOpen(false)
                setDeletingKb(null)
              }}
              disabled={isMutating}
            >
              取消
            </Button>
            <Button variant="destructive" onClick={handleDelete} disabled={isMutating}>
              {deleteMutation.isPending && (
                <Loader2 aria-hidden="true" className="mr-1.5 size-3.5 animate-spin" />
              )}
              确认删除
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
