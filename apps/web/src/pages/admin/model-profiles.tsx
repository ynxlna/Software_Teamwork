import { Cpu, Edit, Loader2, Plus, Trash2 } from 'lucide-react'
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
  useCreateModelProfile,
  useDeleteModelProfile,
  useModelProfiles,
  useUpdateModelProfile,
} from '@/features/admin-config'
import type { ModelProfile } from '@/lib/types'

// ── Constants ──

const PURPOSE_OPTIONS = [
  { value: 'chat', label: '聊天' },
  { value: 'embedding', label: '嵌入' },
  { value: 'rerank', label: '重排序' },
] as const

const PURPOSE_LABELS: Record<string, string> = {
  chat: '聊天',
  embedding: '嵌入',
  rerank: '重排序',
}

const PROVIDER_OPTIONS = [
  { value: 'openai_compatible', label: 'OpenAI兼容' },
  { value: 'siliconflow', label: '硅基流动' },
  { value: 'local_compatible', label: '本地兼容' },
] as const

const PROVIDER_LABELS: Record<string, string> = {
  openai_compatible: 'OpenAI兼容',
  siliconflow: '硅基流动',
  local_compatible: '本地兼容',
}

// ── Types ──

interface FormData {
  name: string
  purpose: string
  provider: string
  baseUrl: string
  model: string
  apiKey: string
  timeoutMs: number
  maxTokens: number
}

type NotificationState = {
  type: 'success' | 'error'
  text: string
}

// ── Defaults ──

const EMPTY_FORM: FormData = {
  name: '',
  purpose: 'chat',
  provider: 'openai_compatible',
  baseUrl: '',
  model: '',
  apiKey: '',
  timeoutMs: 60000,
  maxTokens: 0,
}

// ── Helpers ──

function formToCreateRequest(form: FormData) {
  const params: Record<string, unknown> = {
    name: form.name,
    purpose: form.purpose,
    provider: form.provider,
    baseUrl: form.baseUrl,
    model: form.model,
    apiKey: form.apiKey,
    timeoutMs: form.timeoutMs,
  }
  if (form.maxTokens > 0) {
    params.defaultParameters = { max_tokens: form.maxTokens }
  }
  return params
}

function formToUpdateRequest(form: FormData) {
  const params: Record<string, unknown> = {
    name: form.name,
    provider: form.provider,
    baseUrl: form.baseUrl,
    model: form.model,
    timeoutMs: form.timeoutMs,
  }
  if (form.apiKey) {
    params.apiKey = form.apiKey
  }
  if (form.maxTokens > 0) {
    params.defaultParameters = { max_tokens: form.maxTokens }
  }
  return params
}

// ── Skeleton ──

function ModelProfilesSkeleton() {
  return (
    <div className="animate-pulse space-y-4">
      <div className="flex items-center justify-between">
        <div className="h-7 w-32 rounded bg-muted" />
        <div className="h-8 w-32 rounded bg-muted" />
      </div>
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

export function ModelProfilesPage() {
  // ── State ──
  const [createOpen, setCreateOpen] = useState(false)
  const [editOpen, setEditOpen] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [editingProfile, setEditingProfile] = useState<ModelProfile | null>(null)
  const [deletingProfile, setDeletingProfile] = useState<ModelProfile | null>(null)

  const [form, setForm] = useState<FormData>(EMPTY_FORM)
  const [notification, setNotification] = useState<NotificationState | null>(null)

  // ── Queries & mutations ──

  const { data, isLoading, isError, error, refetch } = useModelProfiles()

  const createMutation = useCreateModelProfile()
  const updateMutation = useUpdateModelProfile()
  const deleteMutation = useDeleteModelProfile()

  const isMutating =
    createMutation.isPending || updateMutation.isPending || deleteMutation.isPending

  // ── Notification auto-dismiss ──

  useEffect(() => {
    if (!notification) return
    const timer = setTimeout(() => setNotification(null), 4000)
    return () => clearTimeout(timer)
  }, [notification])

  // ── Handlers ──

  const updateField = useCallback(
    <K extends keyof FormData>(field: K, value: FormData[K]) => {
      setForm((prev) => ({ ...prev, [field]: value }))
    },
    [],
  )

  const openCreate = useCallback(() => {
    setForm(EMPTY_FORM)
    setCreateOpen(true)
  }, [])

  const openEdit = useCallback((profile: ModelProfile) => {
    setEditingProfile(profile)
    setForm({
      name: profile.name,
      purpose: profile.purpose,
      provider: profile.provider,
      baseUrl: profile.baseUrl,
      model: profile.model,
      apiKey: '',
      timeoutMs: profile.timeoutMs,
      maxTokens: (profile.defaultParameters?.max_tokens as number) ?? 0,
    })
    setEditOpen(true)
  }, [])

  const openDelete = useCallback((profile: ModelProfile) => {
    setDeletingProfile(profile)
    setDeleteOpen(true)
  }, [])

  const handleCreate = useCallback(() => {
    createMutation.mutate(formToCreateRequest(form) as Parameters<typeof createMutation.mutate>[0], {
      onSuccess: () => {
        setNotification({ type: 'success', text: '模型配置创建成功' })
        setCreateOpen(false)
      },
      onError: (err: Error) => {
        setNotification({ type: 'error', text: `创建失败: ${err.message}` })
      },
    })
  }, [form, createMutation])

  const handleEdit = useCallback(() => {
    if (!editingProfile) return
    updateMutation.mutate(
      {
        id: editingProfile.id,
        ...formToUpdateRequest(form),
      } as Parameters<typeof updateMutation.mutate>[0],
      {
        onSuccess: () => {
          setNotification({ type: 'success', text: '模型配置更新成功' })
          setEditOpen(false)
          setEditingProfile(null)
        },
        onError: (err: Error) => {
          setNotification({ type: 'error', text: `更新失败: ${err.message}` })
        },
      },
    )
  }, [editingProfile, form, updateMutation])

  const handleDelete = useCallback(() => {
    if (!deletingProfile) return
    deleteMutation.mutate(deletingProfile.id, {
      onSuccess: () => {
        setNotification({ type: 'success', text: '模型配置已删除' })
        setDeleteOpen(false)
        setDeletingProfile(null)
      },
      onError: (err: Error) => {
        setNotification({ type: 'error', text: `删除失败: ${err.message}` })
      },
    })
  }, [deletingProfile, deleteMutation])

  // ── Derived ──

  const items = data ?? []
  const isEmpty = !isLoading && !isError && items.length === 0

  // ── Render ──

  return (
    <div>
      {/* Header */}
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h3 className="text-2xl font-semibold text-foreground">模型管理</h3>
          <p className="mt-1 text-sm text-muted-foreground">
            管理 AI 模型配置，支持新建、编辑、删除操作。
          </p>
        </div>
        <Button onClick={openCreate}>
          <Plus aria-hidden="true" className="mr-1 size-4" />
          新建模型
        </Button>
      </div>

      {/* Toast notification */}
      {notification && (
        <div
          role="alert"
          className={`toast-enter mb-4 rounded-lg border px-4 py-3 text-sm ${
            notification.type === 'success'
              ? 'border-emerald-500/50 bg-emerald-50 text-emerald-800 dark:border-emerald-400/30 dark:bg-emerald-950 dark:text-emerald-300'
              : 'border-destructive/50 bg-destructive/10 text-destructive'
          }`}
        >
          {notification.text}
        </div>
      )}

      {/* Loading state */}
      {isLoading && <ModelProfilesSkeleton />}

      {/* Error state */}
      {isError && !isLoading && (
        <div className="rounded-lg border border-destructive/50 bg-destructive/10 p-6 text-center">
          <p className="mb-3 text-sm text-destructive">
            加载模型配置失败: {error instanceof Error ? error.message : '未知错误'}
          </p>
          <Button variant="outline" size="sm" onClick={() => refetch()}>
            <Loader2 aria-hidden="true" className="mr-1.5 size-3.5" />
            重试
          </Button>
        </div>
      )}

      {/* Empty state */}
      {isEmpty && (
        <div className="rounded-lg border border-dashed border-border p-12 text-center">
          <Cpu aria-hidden="true" className="mx-auto mb-3 size-10 text-muted-foreground/40" />
          <p className="text-sm text-muted-foreground">
            暂无模型配置，点击新建模型开始
          </p>
          <Button variant="outline" size="sm" className="mt-3" onClick={openCreate}>
            <Plus aria-hidden="true" className="mr-1 size-3.5" />
            新建模型
          </Button>
        </div>
      )}

      {/* Table */}
      {!isLoading && !isError && items.length > 0 && (
        <div className="overflow-x-auto rounded-lg border border-border bg-card">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border bg-muted/30">
                <th className="px-4 py-2.5 text-left font-medium text-muted-foreground">
                  名称
                </th>
                <th className="px-4 py-2.5 text-left font-medium text-muted-foreground">
                  用途
                </th>
                <th className="hidden px-4 py-2.5 text-left font-medium text-muted-foreground sm:table-cell">
                  服务商
                </th>
                <th className="hidden px-4 py-2.5 text-left font-medium text-muted-foreground md:table-cell">
                  模型名称
                </th>
                <th className="px-4 py-2.5 text-left font-medium text-muted-foreground">
                  API Key
                </th>
                <th className="px-4 py-2.5 text-left font-medium text-muted-foreground">
                  状态
                </th>
                <th className="px-4 py-2.5 text-right font-medium text-muted-foreground">
                  操作
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {items.map((profile) => (
                <tr
                  key={profile.id}
                  className="transition-colors duration-150 hover:bg-muted/30"
                >
                  <td className="max-w-36 truncate px-4 py-2.5 font-medium text-foreground">
                    {profile.name}
                  </td>
                  <td className="px-4 py-2.5">
                    <Badge variant="secondary">
                      {PURPOSE_LABELS[profile.purpose] ?? profile.purpose}
                    </Badge>
                  </td>
                  <td className="hidden px-4 py-2.5 text-muted-foreground sm:table-cell">
                    {PROVIDER_LABELS[profile.provider] ?? profile.provider}
                  </td>
                  <td className="hidden max-w-40 truncate px-4 py-2.5 text-muted-foreground md:table-cell">
                    {profile.model}
                  </td>
                  <td className="px-4 py-2.5">
                    {profile.apiKeyConfigured ? (
                      <Badge variant="default">已配置</Badge>
                    ) : (
                      <Badge variant="outline">未配置</Badge>
                    )}
                  </td>
                  <td className="px-4 py-2.5">
                    {profile.enabled ? (
                      <Badge variant="default">启用</Badge>
                    ) : (
                      <Badge variant="secondary">禁用</Badge>
                    )}
                  </td>
                  <td className="px-4 py-2.5">
                    <div className="flex items-center justify-end gap-1">
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        onClick={() => openEdit(profile)}
                        aria-label={`编辑 ${profile.name}`}
                      >
                        <Edit aria-hidden="true" className="size-3.5" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        onClick={() => openDelete(profile)}
                        aria-label={`删除 ${profile.name}`}
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
      )}

      {/* ── Create Dialog ── */}
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>新建模型配置</DialogTitle>
            <DialogDescription>填写模型提供商和连接信息。</DialogDescription>
          </DialogHeader>

          <div className="space-y-3">
            {/* Name */}
            <div>
              <label
                htmlFor="mp-create-name"
                className="mb-1 block text-sm font-medium text-foreground"
              >
                名称 <span className="text-destructive">*</span>
              </label>
              <Input
                id="mp-create-name"
                type="text"
                placeholder="模型配置名称"
                value={form.name}
                onChange={(e) => updateField('name', e.target.value)}
              />
            </div>

            {/* Purpose */}
            <div>
              <label
                htmlFor="mp-create-purpose"
                className="mb-1 block text-sm font-medium text-foreground"
              >
                用途 <span className="text-destructive">*</span>
              </label>
              <select
                id="mp-create-purpose"
                value={form.purpose}
                onChange={(e) => updateField('purpose', e.target.value)}
                className="h-8 w-full rounded-lg border border-input bg-transparent px-2.5 py-1 text-sm text-foreground transition-colors outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 md:text-sm"
              >
                {PURPOSE_OPTIONS.map((opt) => (
                  <option key={opt.value} value={opt.value}>
                    {opt.label}
                  </option>
                ))}
              </select>
            </div>

            {/* Provider */}
            <div>
              <label
                htmlFor="mp-create-provider"
                className="mb-1 block text-sm font-medium text-foreground"
              >
                服务商 <span className="text-destructive">*</span>
              </label>
              <select
                id="mp-create-provider"
                value={form.provider}
                onChange={(e) => updateField('provider', e.target.value)}
                className="h-8 w-full rounded-lg border border-input bg-transparent px-2.5 py-1 text-sm text-foreground transition-colors outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 md:text-sm"
              >
                {PROVIDER_OPTIONS.map((opt) => (
                  <option key={opt.value} value={opt.value}>
                    {opt.label}
                  </option>
                ))}
              </select>
            </div>

            {/* Base URL */}
            <div>
              <label
                htmlFor="mp-create-baseurl"
                className="mb-1 block text-sm font-medium text-foreground"
              >
                Base URL <span className="text-destructive">*</span>
              </label>
              <Input
                id="mp-create-baseurl"
                type="url"
                placeholder="https://api.example.com/v1"
                value={form.baseUrl}
                onChange={(e) => updateField('baseUrl', e.target.value)}
              />
            </div>

            {/* Model Name */}
            <div>
              <label
                htmlFor="mp-create-model"
                className="mb-1 block text-sm font-medium text-foreground"
              >
                模型名称 <span className="text-destructive">*</span>
              </label>
              <Input
                id="mp-create-model"
                type="text"
                placeholder="gpt-4o"
                value={form.model}
                onChange={(e) => updateField('model', e.target.value)}
              />
            </div>

            {/* API Key */}
            <div>
              <label
                htmlFor="mp-create-apikey"
                className="mb-1 block text-sm font-medium text-foreground"
              >
                API Key <span className="text-destructive">*</span>
              </label>
              <Input
                id="mp-create-apikey"
                type="password"
                placeholder="sk-..."
                value={form.apiKey}
                onChange={(e) => updateField('apiKey', e.target.value)}
              />
            </div>

            {/* Timeout (ms) */}
            <div>
              <label
                htmlFor="mp-create-timeout"
                className="mb-1 block text-sm font-medium text-foreground"
              >
                超时时间 (毫秒)
              </label>
              <Input
                id="mp-create-timeout"
                type="number"
                placeholder="60000"
                value={form.timeoutMs}
                onChange={(e) =>
                  updateField('timeoutMs', Math.max(1000, Number(e.target.value)))
                }
              />
            </div>

            {/* Max Tokens */}
            <div>
              <label
                htmlFor="mp-create-maxtokens"
                className="mb-1 block text-sm font-medium text-foreground"
              >
                最大 Token 数
              </label>
              <Input
                id="mp-create-maxtokens"
                type="number"
                placeholder="0 表示不限制"
                value={form.maxTokens}
                onChange={(e) =>
                  updateField('maxTokens', Math.max(0, Number(e.target.value)))
                }
              />
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)} disabled={isMutating}>
              取消
            </Button>
            <Button
              onClick={handleCreate}
              disabled={!form.name.trim() || !form.apiKey.trim() || isMutating}
            >
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
            <DialogTitle>编辑模型配置</DialogTitle>
            <DialogDescription>
              修改 "{editingProfile?.name}" 的配置信息。
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-3">
            {/* Name */}
            <div>
              <label
                htmlFor="mp-edit-name"
                className="mb-1 block text-sm font-medium text-foreground"
              >
                名称 <span className="text-destructive">*</span>
              </label>
              <Input
                id="mp-edit-name"
                type="text"
                placeholder="模型配置名称"
                value={form.name}
                onChange={(e) => updateField('name', e.target.value)}
              />
            </div>

            {/* Purpose (read-only on edit) */}
            <div>
              <label
                htmlFor="mp-edit-purpose"
                className="mb-1 block text-sm font-medium text-foreground"
              >
                用途
              </label>
              <Input
                id="mp-edit-purpose"
                type="text"
                value={PURPOSE_LABELS[form.purpose] ?? form.purpose}
                disabled
                className="text-muted-foreground"
              />
            </div>

            {/* Provider */}
            <div>
              <label
                htmlFor="mp-edit-provider"
                className="mb-1 block text-sm font-medium text-foreground"
              >
                服务商 <span className="text-destructive">*</span>
              </label>
              <select
                id="mp-edit-provider"
                value={form.provider}
                onChange={(e) => updateField('provider', e.target.value)}
                className="h-8 w-full rounded-lg border border-input bg-transparent px-2.5 py-1 text-sm text-foreground transition-colors outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 md:text-sm"
              >
                {PROVIDER_OPTIONS.map((opt) => (
                  <option key={opt.value} value={opt.value}>
                    {opt.label}
                  </option>
                ))}
              </select>
            </div>

            {/* Base URL */}
            <div>
              <label
                htmlFor="mp-edit-baseurl"
                className="mb-1 block text-sm font-medium text-foreground"
              >
                Base URL <span className="text-destructive">*</span>
              </label>
              <Input
                id="mp-edit-baseurl"
                type="url"
                placeholder="https://api.example.com/v1"
                value={form.baseUrl}
                onChange={(e) => updateField('baseUrl', e.target.value)}
              />
            </div>

            {/* Model Name */}
            <div>
              <label
                htmlFor="mp-edit-model"
                className="mb-1 block text-sm font-medium text-foreground"
              >
                模型名称 <span className="text-destructive">*</span>
              </label>
              <Input
                id="mp-edit-model"
                type="text"
                placeholder="gpt-4o"
                value={form.model}
                onChange={(e) => updateField('model', e.target.value)}
              />
            </div>

            {/* API Key */}
            <div>
              <label
                htmlFor="mp-edit-apikey"
                className="mb-1 block text-sm font-medium text-foreground"
              >
                API Key
              </label>
              <Input
                id="mp-edit-apikey"
                type="password"
                placeholder="留空保持不变"
                value={form.apiKey}
                onChange={(e) => updateField('apiKey', e.target.value)}
              />
              <p className="mt-1 text-xs text-muted-foreground">
                留空则保持现有密钥不变。输入新值将替换密钥。
              </p>
            </div>

            {/* Timeout (ms) */}
            <div>
              <label
                htmlFor="mp-edit-timeout"
                className="mb-1 block text-sm font-medium text-foreground"
              >
                超时时间 (毫秒)
              </label>
              <Input
                id="mp-edit-timeout"
                type="number"
                placeholder="60000"
                value={form.timeoutMs}
                onChange={(e) =>
                  updateField('timeoutMs', Math.max(1000, Number(e.target.value)))
                }
              />
            </div>

            {/* Max Tokens */}
            <div>
              <label
                htmlFor="mp-edit-maxtokens"
                className="mb-1 block text-sm font-medium text-foreground"
              >
                最大 Token 数
              </label>
              <Input
                id="mp-edit-maxtokens"
                type="number"
                placeholder="0 表示不限制"
                value={form.maxTokens}
                onChange={(e) =>
                  updateField('maxTokens', Math.max(0, Number(e.target.value)))
                }
              />
            </div>
          </div>

          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setEditOpen(false)
                setEditingProfile(null)
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
              确定要删除模型配置 "{deletingProfile?.name}" 吗？此操作不可撤销。
            </DialogDescription>
          </DialogHeader>

          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setDeleteOpen(false)
                setDeletingProfile(null)
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
