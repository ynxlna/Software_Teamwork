import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'

import { createLLMConfigVersion, getCurrentLLMConfig, testLLMConnection } from '@/api/admin'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

interface FormData {
  profileId: string
  modelName: string
  timeoutSeconds: number
  temperature: number
  maxTokens: number
}

interface NotificationState {
  type: 'success' | 'error'
  text: string
}

export function SystemSettings() {
  const queryClient = useQueryClient()
  const [notification, setNotification] = useState<NotificationState | null>(null)
  const [formInitialized, setFormInitialized] = useState(false)

  // Draft form state
  const [form, setForm] = useState<FormData>({
    profileId: '',
    modelName: '',
    timeoutSeconds: 30,
    temperature: 0.7,
    maxTokens: 4096,
  })

  // Fetch current config
  const { data, isLoading, isError, error } = useQuery({
    queryKey: ['admin', 'llm-config'],
    queryFn: getCurrentLLMConfig,
    staleTime: 30_000,
  })

  // Sync form from fetched data (only on first load)
  useEffect(() => {
    if (data && !formInitialized) {
      setForm({
        profileId: data.profileId ?? '',
        modelName: data.modelName ?? '',
        timeoutSeconds: data.timeoutSeconds ?? 30,
        temperature: data.temperature ?? 0.7,
        maxTokens: data.maxTokens ?? 4096,
      })
      setFormInitialized(true)
    }
  }, [data, formInitialized])

  // Notification auto-dismiss
  useEffect(() => {
    if (!notification) return
    const timer = setTimeout(() => setNotification(null), 4000)
    return () => clearTimeout(timer)
  }, [notification])

  // Save mutation
  const saveMutation = useMutation({
    mutationFn: (payload: FormData) =>
      createLLMConfigVersion({
        provider: 'ai-gateway',
        profileId: payload.profileId,
        modelName: payload.modelName,
        timeoutSeconds: payload.timeoutSeconds,
        temperature: payload.temperature,
        maxTokens: payload.maxTokens,
        activate: true,
      }),
    onSuccess: () => {
      setNotification({ type: 'success', text: '配置已保存' })
      queryClient.invalidateQueries({ queryKey: ['admin', 'llm-config'] })
    },
    onError: (err: Error) => {
      setNotification({ type: 'error', text: `保存失败: ${err.message}` })
    },
  })

  // Test connection mutation
  const testMutation = useMutation({
    mutationFn: (payload: { profileId: string; modelName: string }) =>
      testLLMConnection({
        provider: 'ai-gateway',
        profileId: payload.profileId,
        modelName: payload.modelName,
      }),
    onSuccess: (res) => {
      setNotification({
        type: 'success',
        text: res.success
          ? `连接成功！延迟 ${res.latencyMs ?? '?'}ms，模型 ${res.modelName ?? '未知'}`
          : `连接失败: ${res.errorMessage ?? '未知错误'}`,
      })
    },
    onError: (err: Error) => {
      setNotification({ type: 'error', text: `连接测试失败: ${err.message}` })
    },
  })

  const updateField = useCallback((field: keyof FormData, value: string | number) => {
    setForm((prev) => ({ ...prev, [field]: value }))
  }, [])

  const handleSave = () => {
    saveMutation.mutate(form)
  }

  const handleTest = () => {
    testMutation.mutate({
      profileId: form.profileId,
      modelName: form.modelName,
    })
  }

  // Loading state
  if (isLoading) {
    return (
      <div>
        <h3 className="mb-4 text-2xl font-semibold text-foreground">系统设置</h3>
        <p className="mb-6 text-sm text-muted-foreground">
          全局系统配置，包括 LLM API 连接、向量数据库连接、系统参数等。
        </p>
        <div className="animate-pulse space-y-4 rounded-lg border border-border bg-card p-6">
          <div className="h-5 w-24 rounded bg-muted" />
          <div className="grid grid-cols-2 gap-4">
            {Array.from({ length: 4 }).map((_, i) => (
              <div key={i}>
                <div className="mb-1 h-4 w-16 rounded bg-muted" />
                <div className="h-8 w-full rounded bg-muted" />
              </div>
            ))}
          </div>
          <div className="flex gap-2">
            <div className="h-8 w-24 rounded bg-muted" />
            <div className="h-8 w-24 rounded bg-muted" />
          </div>
        </div>
      </div>
    )
  }

  // Error state
  if (isError) {
    return (
      <div>
        <h3 className="mb-4 text-2xl font-semibold text-foreground">系统设置</h3>
        <div className="rounded-lg border border-destructive/50 bg-destructive/10 p-6 text-center">
          <p className="text-sm text-destructive">
            加载配置失败: {error instanceof Error ? error.message : '未知错误'}
          </p>
        </div>
      </div>
    )
  }

  return (
    <div>
      <h3 className="mb-4 text-2xl font-semibold text-foreground">系统设置</h3>
      <p className="mb-6 text-sm text-muted-foreground">
        全局系统配置，包括 LLM API 连接、向量数据库连接、系统参数等。
      </p>

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

      {/* LLM Config Form */}
      <div className="rounded-lg border border-border bg-card p-6 hover:shadow-sm transition-shadow duration-200">
        <h4 className="mb-5 text-lg font-semibold text-foreground">LLM 配置</h4>

        <div className="grid grid-cols-2 gap-4">
          {/* Profile ID */}
          <div>
            <label
              htmlFor="llm-profile-id"
              className="mb-1.5 block text-sm font-medium text-foreground"
            >
              AI Gateway 模型配置 ID
            </label>
            <Input
              id="llm-profile-id"
              type="text"
              placeholder="model-profile-id"
              value={form.profileId}
              onChange={(e) => updateField('profileId', e.target.value)}
            />
          </div>

          {/* Model Name */}
          <div>
            <label
              htmlFor="llm-model-name"
              className="mb-1.5 block text-sm font-medium text-foreground"
            >
              模型名称
            </label>
            <Input
              id="llm-model-name"
              type="text"
              placeholder="gpt-4o"
              value={form.modelName}
              onChange={(e) => updateField('modelName', e.target.value)}
            />
          </div>

          {/* Timeout */}
          <div>
            <label
              htmlFor="llm-timeout"
              className="mb-1.5 block text-sm font-medium text-foreground"
            >
              超时时间（秒）
            </label>
            <Input
              id="llm-timeout"
              type="number"
              placeholder="30"
              min={1}
              max={300}
              value={form.timeoutSeconds}
              onChange={(e) => updateField('timeoutSeconds', Number(e.target.value))}
            />
          </div>

          {/* Temperature */}
          <div>
            <label
              htmlFor="llm-temperature"
              className="mb-1.5 block text-sm font-medium text-foreground"
            >
              温度
            </label>
            <Input
              id="llm-temperature"
              type="number"
              placeholder="0.7"
              min={0}
              max={2}
              step={0.1}
              value={form.temperature}
              onChange={(e) => updateField('temperature', Number(e.target.value))}
            />
          </div>

          {/* Max Tokens */}
          <div className="col-span-2">
            <label
              htmlFor="llm-max-tokens"
              className="mb-1.5 block text-sm font-medium text-foreground"
            >
              最大 Token 数
            </label>
            <Input
              id="llm-max-tokens"
              type="number"
              placeholder="4096"
              min={1}
              max={128000}
              value={form.maxTokens}
              onChange={(e) => updateField('maxTokens', Number(e.target.value))}
            />
          </div>
        </div>

        {/* Action buttons */}
        <div className="mt-5 flex gap-2">
          <Button onClick={handleSave} disabled={saveMutation.isPending}>
            {saveMutation.isPending && (
              <Loader2 aria-hidden="true" className="mr-1.5 size-3.5 animate-spin" />
            )}
            保存配置
          </Button>
          <Button variant="outline" onClick={handleTest} disabled={testMutation.isPending}>
            {testMutation.isPending && (
              <Loader2 aria-hidden="true" className="mr-1.5 size-3.5 animate-spin" />
            )}
            测试连接
          </Button>
        </div>
      </div>
    </div>
  )
}
