import { Ban, Download, FileText, Loader2, PencilLine, Play, RefreshCw, Save } from 'lucide-react'
import { type FormEvent, useEffect, useMemo, useState } from 'react'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type {
  CreateReportFormValues,
  Report,
  ReportFile,
  ReportJob,
  ReportJobStatus,
  ReportMaterial,
  ReportOutlineNode,
  ReportSection,
  ReportSectionVersion,
  ReportTemplate,
  ReportType,
} from '@/features/reports'
import {
  createReportSchema,
  defaultCreateReportValues,
  useCancelReportJob,
  useCreateReportFileMutation,
  useCreateReportJobMutation,
  useCreateReportMutation,
  useDownloadReportFileMutation,
  useReportBootstrapQueries,
  useReportDetailQueries,
  useReportEvents,
  useReportJobQuery,
  useRetryReportJobMutation,
  useSectionVersions,
  useUpdateReportOutlineMutation,
  useUpdateReportSectionMutation,
} from '@/features/reports'
import { cn } from '@/lib/utils'

const fallbackTypes: ReportType[] = [
  {
    code: 'summer_peak_inspection',
    name: '迎峰度夏检查报告',
    description: '面向高温高负荷期间的设备安全检查和风险闭环。',
    enabled: true,
    defaultTemplateId: 'tpl-local-summer',
  },
  {
    code: 'coal_inventory_audit',
    name: '煤库存审计报告',
    description: '面向煤库存、台账、盘点差异和风险控制的审计报告。',
    enabled: true,
    defaultTemplateId: 'tpl-local-coal',
  },
]

const fallbackTemplates: ReportTemplate[] = [
  {
    id: 'tpl-local-summer',
    templateName: '迎峰度夏默认模板',
    reportType: 'summer_peak_inspection',
    version: 1,
    enabled: true,
    filename: 'summer-peak-template.docx',
    createdAt: '2026-06-28T10:00:00Z',
  },
  {
    id: 'tpl-local-coal',
    templateName: '煤库存审计模板',
    reportType: 'coal_inventory_audit',
    version: 1,
    enabled: true,
    filename: 'coal-inventory-template.docx',
    createdAt: '2026-06-28T10:00:00Z',
  },
]

const fallbackMaterials: ReportMaterial[] = [
  {
    id: 'mat-equipment-ledger',
    materialName: '设备运行台账与缺陷闭环记录',
    materialType: 'plant_report',
    category: '运行资料',
    enabled: true,
    createdAt: '2026-06-28T10:00:00Z',
  },
  {
    id: 'mat-risk-standard',
    materialName: '迎峰度夏风险检查标准',
    materialType: 'technical_doc',
    category: '技术标准',
    enabled: true,
    createdAt: '2026-06-28T10:00:00Z',
  },
]

const fallbackOutline: ReportOutlineNode[] = [
  { id: 'local-outline-1', title: '前言', level: 1, numbering: '1' },
  { id: 'local-outline-2', title: '检查背景与依据', level: 1, numbering: '2' },
  { id: 'local-outline-21', title: '检查范围', level: 2, numbering: '2.1' },
  { id: 'local-outline-22', title: '资料来源', level: 2, numbering: '2.2' },
  { id: 'local-outline-3', title: '重点问题分析', level: 1, numbering: '3' },
  { id: 'local-outline-4', title: '整改建议', level: 1, numbering: '4' },
]

const fallbackSections: ReportSection[] = fallbackOutline.map((node, index) => ({
  id: `local-section-${index + 1}`,
  reportId: 'local-report',
  outlineNodeId: node.id,
  title: node.title,
  level: node.level,
  numbering: node.numbering,
  content:
    index === 0
      ? '为保障迎峰度夏期间电力设备安全稳定运行，本次检查围绕设备健康状态、隐患治理闭环、运行风险管控和应急保障能力开展。'
      : '本章节内容将在生成正文后写入，也可由报告编写人先行维护章节要求。',
  tables: [],
  generationStatus: index === 5 ? 'failed' : 'succeeded',
  contentSource: index === 0 ? 'mixed' : 'ai',
  manualEdited: false,
}))

const fallbackInitialSection = fallbackSections[0] ?? {
  id: 'local-section-empty',
  reportId: 'local-report',
  title: '章节正文',
  level: 1,
  content: '',
  tables: [],
  generationStatus: 'pending' as const,
}

const steps = [
  { key: 'draft', label: '1. 草稿与大纲' },
  { key: 'outline', label: '2. 编辑大纲' },
  { key: 'content', label: '3. 正文生成' },
  { key: 'export', label: '4. DOCX 导出' },
] as const

type StepKey = (typeof steps)[number]['key']

const statusText: Record<ReportJobStatus, string> = {
  pending: '等待中',
  running: '生成中',
  succeeded: '已完成',
  partial_succeeded: '部分成功',
  failed: '失败',
  canceled: '已取消',
}

function getProgressPercent(job?: ReportJob | null): number {
  const value = job?.progress?.percent
  return typeof value === 'number' ? Math.max(0, Math.min(100, value)) : 0
}

function formatDate(value?: string): string {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

export function ReportGeneratePage() {
  const [step, setStep] = useState<StepKey>('draft')
  const [form, setForm] = useState<CreateReportFormValues>({
    ...defaultCreateReportValues,
  })
  const [selectedMaterialIds, setSelectedMaterialIds] = useState<string[]>(['mat-equipment-ledger'])
  const [currentReport, setCurrentReport] = useState<Report | null>(null)
  const [activeJobId, setActiveJobId] = useState<string | null>(null)
  const [lastJob, setLastJob] = useState<ReportJob | null>(null)
  const [latestFile, setLatestFile] = useState<ReportFile | null>(null)
  const [activeSectionId, setActiveSectionId] = useState(fallbackInitialSection.id)
  const [sectionDraft, setSectionDraft] = useState(fallbackInitialSection.content ?? '')
  const [showVersions, setShowVersions] = useState(false)
  const [notice, setNotice] = useState<string | null>(null)
  const [formError, setFormError] = useState<string | null>(null)

  const { typeQuery, templateQuery, materialQuery } = useReportBootstrapQueries(form.reportType)
  const { outlinesQuery, sectionsQuery } = useReportDetailQueries(currentReport?.id ?? null)
  const jobQuery = useReportJobQuery(activeJobId)
  const createReportMutation = useCreateReportMutation()
  const createJobMutation = useCreateReportJobMutation()
  const saveOutlineMutation = useUpdateReportOutlineMutation(currentReport?.id ?? '')
  const saveSectionMutation = useUpdateReportSectionMutation(currentReport?.id ?? '')
  const createFileMutation = useCreateReportFileMutation()
  const retryJobMutation = useRetryReportJobMutation()
  const downloadMutation = useDownloadReportFileMutation()
  const cancelJobMutation = useCancelReportJob()
  const eventsQuery = useReportEvents(currentReport?.id ?? null)
  const sectionVersionsQuery = useSectionVersions(
    currentReport?.id ?? null,
    showVersions ? activeSectionId : null,
  )

  const reportTypes = typeQuery.data?.length ? typeQuery.data : fallbackTypes
  const templates = templateQuery.data?.items.length
    ? templateQuery.data.items
    : fallbackTemplates.filter((template) => template.reportType === form.reportType)
  const materials = materialQuery.data?.items.length ? materialQuery.data.items : fallbackMaterials
  const outline = outlinesQuery.data?.[0]?.sections ?? fallbackOutline
  const sections = sectionsQuery.data?.length ? sectionsQuery.data : fallbackSections
  const activeSection = sections.find((item) => item.id === activeSectionId) ?? sections[0]
  const effectiveJob = jobQuery.data ?? lastJob
  const selectedTemplate = templates.find((template) => template.id === form.templateId)

  const contractWarning = useMemo(() => {
    if (typeQuery.isError || templateQuery.isError || materialQuery.isError) {
      return 'gateway 暂未联通或未返回报告配置，当前展示本地原型数据；请求路径仍按 /api/v1 最新契约组织。'
    }
    return null
  }, [materialQuery.isError, templateQuery.isError, typeQuery.isError])

  useEffect(() => {
    const firstTemplate = templates[0]
    if (!form.templateId && firstTemplate) {
      setForm((prev) => ({ ...prev, templateId: firstTemplate.id }))
    }
  }, [form.templateId, templates])

  useEffect(() => {
    if (activeSection) {
      setSectionDraft(activeSection.content ?? '')
    }
  }, [activeSection])

  useEffect(() => {
    if (jobQuery.data) {
      setLastJob(jobQuery.data)
    }
  }, [jobQuery.data])

  const updateForm = (field: keyof CreateReportFormValues, value: string | number) => {
    setForm((prev) => ({ ...prev, [field]: value }))
  }

  const toggleMaterial = (id: string) => {
    setSelectedMaterialIds((prev) =>
      prev.includes(id) ? prev.filter((item) => item !== id) : [...prev, id],
    )
  }

  const createLocalReport = (): Report => ({
    id: `local-report-${Date.now().toString(36)}`,
    name: form.name,
    reportType: form.reportType,
    templateId: form.templateId,
    topic: form.topic,
    specialty: form.specialty,
    businessObject: form.businessObject,
    year: form.year,
    status: 'outline_generated',
    source: 'frontend',
    createdAt: new Date().toISOString(),
  })

  const createLocalJob = (jobType: ReportJob['jobType']): ReportJob => ({
    id: `local-job-${Date.now().toString(36)}`,
    reportId: currentReport?.id ?? 'local-report',
    jobType,
    status: jobType === 'content_generation' ? 'partial_succeeded' : 'succeeded',
    progress:
      jobType === 'content_generation'
        ? { completedSections: 5, totalSections: 6, percent: 84 }
        : { completedSections: 6, totalSections: 6, percent: 100 },
    resultSummary:
      jobType === 'content_generation'
        ? '正文已生成，末章可通过单章重试或任务重试继续处理。'
        : '大纲已按模板结构创建。',
    createdAt: new Date().toISOString(),
  })

  const handleCreateReport = async (event: FormEvent) => {
    event.preventDefault()
    setFormError(null)
    setNotice(null)

    const parsed = createReportSchema.safeParse(form)
    if (!parsed.success) {
      setFormError(parsed.error.issues[0]?.message ?? '请检查报告参数')
      return
    }

    const payload = {
      name: parsed.data.name,
      reportType: parsed.data.reportType,
      templateId: parsed.data.templateId,
      topic: parsed.data.topic,
      specialty: parsed.data.specialty,
      businessObject: parsed.data.businessObject,
      year: parsed.data.year,
      extraContext: parsed.data.extraContextText
        ? { note: parsed.data.extraContextText }
        : undefined,
      source: 'frontend' as const,
    }

    try {
      const report = await createReportMutation.mutateAsync(payload)
      setCurrentReport(report)
      const job = await createJobMutation.mutateAsync({
        reportId: report.id,
        payload: {
          jobType: 'outline_generation',
          target: { scope: 'outline' },
          materialIds: selectedMaterialIds,
          requirements: parsed.data.extraContextText,
        },
      })
      setLastJob(job)
      setActiveJobId(job.id)
      setStep('outline')
      setNotice('已创建报告草稿，并通过 /api/v1/reports/{reportId}/jobs 创建大纲任务。')
    } catch (error) {
      const report = createLocalReport()
      const job = createLocalJob('outline_generation')
      setCurrentReport(report)
      setLastJob(job)
      setActiveJobId(null)
      setStep('outline')
      setNotice(
        error instanceof Error
          ? `接口未联通，已进入本地原型流程：${error.message}`
          : '接口未联通，已进入本地原型流程。',
      )
    }
  }

  const handleSaveOutline = async () => {
    if (!currentReport || !outlinesQuery.data?.[0]) {
      setNotice(
        '当前使用本地大纲示例；真实保存将调用 PATCH /api/v1/reports/{reportId}/outlines/{outlineId}。',
      )
      return
    }

    await saveOutlineMutation.mutateAsync({
      outlineId: outlinesQuery.data[0].id,
      sections: outline,
    })
    setNotice('大纲已保存，后端将负责重新编号和结构合法性校验。')
  }

  const handleGenerateContent = async () => {
    if (!currentReport || currentReport.id.startsWith('local-')) {
      const job = createLocalJob('content_generation')
      setLastJob(job)
      setStep('content')
      setNotice('当前使用本地正文生成演示；真实流程会创建 content_generation 任务并轮询任务状态。')
      return
    }

    const job = await createJobMutation.mutateAsync({
      reportId: currentReport.id,
      payload: {
        jobType: 'content_generation',
        target: { scope: 'report' },
        materialIds: selectedMaterialIds,
        options: { preserveManualEdits: true, saveResult: true },
      },
    })
    setLastJob(job)
    setActiveJobId(job.id)
    setStep('content')
  }

  const handleSaveSection = async () => {
    if (!currentReport || !activeSection || activeSection.id.startsWith('local-')) {
      setNotice(
        '当前使用本地章节内容；真实保存将调用 PATCH /api/v1/reports/{reportId}/sections/{sectionId}。',
      )
      return
    }

    await saveSectionMutation.mutateAsync({
      sectionId: activeSection.id,
      title: activeSection.title,
      content: sectionDraft,
    })
    setNotice('章节正文已保存。')
  }

  const handleRetry = async () => {
    if (lastJob?.id && !lastJob.id.startsWith('local-')) {
      const attempt = await retryJobMutation.mutateAsync(lastJob.id)
      setNotice(`已创建重试尝试：${attempt.id}`)
      return
    }
    setNotice(
      '本地演示中已保留失败章节，可在真实接口联通后通过 POST /api/v1/report-jobs/{jobId}/attempts 重试。',
    )
  }

  const handleCancel = async () => {
    if (!effectiveJob?.id || effectiveJob.id.startsWith('local-')) {
      setNotice('本地演示任务无需取消。')
      return
    }
    if (effectiveJob.status !== 'pending' && effectiveJob.status !== 'running') {
      setNotice('只有等待中或运行中的任务才能取消。')
      return
    }
    try {
      await cancelJobMutation.mutateAsync(effectiveJob.id)
      setNotice('已请求取消任务。')
    } catch {
      setNotice('任务取消暂不支持（Gateway 契约待补齐）。')
    }
  }

  const handleExport = async () => {
    if (!currentReport || currentReport.id.startsWith('local-')) {
      const file: ReportFile = {
        id: `local-file-${Date.now().toString(36)}`,
        reportId: currentReport?.id ?? 'local-report',
        filename: `${form.name}.docx`,
        format: 'docx',
        status: 'succeeded',
        createdAt: new Date().toISOString(),
      }
      setLatestFile(file)
      setStep('export')
      setNotice('当前使用本地导出演示；真实流程会 POST /api/v1/report-files 创建 DOCX 文件资源。')
      return
    }

    const file = await createFileMutation.mutateAsync({
      reportId: currentReport.id,
      format: 'docx',
      templateId: selectedTemplate?.id,
      styleOptions: { numberingMode: 'global' },
    })
    setLatestFile(file)
    setStep('export')
  }

  const handleDownload = async () => {
    if (!latestFile || latestFile.id.startsWith('local-')) {
      setNotice(
        '本地演示文件没有真实二进制内容；真实下载将调用 GET /api/v1/report-files/{reportFileId}/content。',
      )
      return
    }

    const blob = await downloadMutation.mutateAsync(latestFile.id)
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = latestFile.filename ?? `${form.name}.docx`
    anchor.click()
    URL.revokeObjectURL(url)
  }

  const progressPercent = getProgressPercent(effectiveJob)

  return (
    <div className="flex h-full flex-col overflow-auto bg-background">
      <div className="border-b border-border bg-muted/30 px-6 py-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 className="text-xl font-semibold text-foreground">报告生成</h1>
            <p className="mt-1 text-sm text-muted-foreground">
              按最新 gateway RESTful 契约整合：草稿、大纲、正文任务和 DOCX 文件资源。
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            {steps.map((item) => (
              <Button
                key={item.key}
                type="button"
                variant={step === item.key ? 'default' : 'outline'}
                size="sm"
                onClick={() => setStep(item.key)}
              >
                {item.label}
              </Button>
            ))}
          </div>
        </div>

        {(contractWarning || notice || formError) && (
          <div
            className={cn(
              'mt-4 rounded-lg border px-4 py-3 text-sm',
              formError
                ? 'border-destructive/40 bg-destructive/10 text-destructive'
                : 'border-border bg-card text-muted-foreground',
            )}
          >
            {formError ?? notice ?? contractWarning}
          </div>
        )}
      </div>

      <div className="grid flex-1 gap-6 p-6 xl:grid-cols-[minmax(0,1.1fr)_360px]">
        <div className="min-w-0 space-y-6">
          {step === 'draft' && (
            <form
              className="rounded-lg border border-border bg-card p-5"
              onSubmit={handleCreateReport}
            >
              <div className="mb-5 flex items-center gap-2">
                <FileText className="size-4 text-muted-foreground" />
                <h2 className="text-base font-semibold">创建草稿并生成大纲</h2>
              </div>

              <div className="grid gap-4 md:grid-cols-2">
                <label className="space-y-1.5 text-sm">
                  <span className="font-medium">报告名称</span>
                  <Input
                    value={form.name}
                    onChange={(event) => updateForm('name', event.target.value)}
                  />
                </label>
                <label className="space-y-1.5 text-sm">
                  <span className="font-medium">报告类型</span>
                  <select
                    className="h-8 w-full rounded-lg border border-input bg-background px-2.5 text-sm"
                    value={form.reportType}
                    onChange={(event) => {
                      updateForm('reportType', event.target.value)
                      updateForm('templateId', '')
                    }}
                  >
                    {reportTypes.map((type) => (
                      <option key={type.code} value={type.code}>
                        {type.name}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="space-y-1.5 text-sm">
                  <span className="font-medium">报告模板</span>
                  <select
                    className="h-8 w-full rounded-lg border border-input bg-background px-2.5 text-sm"
                    value={form.templateId}
                    onChange={(event) => updateForm('templateId', event.target.value)}
                  >
                    {templates.map((template) => (
                      <option key={template.id} value={template.id}>
                        {template.templateName} v{template.version}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="space-y-1.5 text-sm">
                  <span className="font-medium">年份</span>
                  <Input
                    type="number"
                    value={form.year}
                    onChange={(event) => updateForm('year', Number(event.target.value))}
                  />
                </label>
                <label className="space-y-1.5 text-sm">
                  <span className="font-medium">专业</span>
                  <Input
                    value={form.specialty ?? ''}
                    onChange={(event) => updateForm('specialty', event.target.value)}
                  />
                </label>
                <label className="space-y-1.5 text-sm">
                  <span className="font-medium">业务对象</span>
                  <Input
                    value={form.businessObject ?? ''}
                    onChange={(event) => updateForm('businessObject', event.target.value)}
                  />
                </label>
                <label className="space-y-1.5 text-sm md:col-span-2">
                  <span className="font-medium">报告主题</span>
                  <Input
                    value={form.topic}
                    onChange={(event) => updateForm('topic', event.target.value)}
                  />
                </label>
                <label className="space-y-1.5 text-sm md:col-span-2">
                  <span className="font-medium">补充上下文 / 生成要求</span>
                  <textarea
                    className="min-h-24 w-full rounded-lg border border-input bg-background px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
                    value={form.extraContextText ?? ''}
                    onChange={(event) => updateForm('extraContextText', event.target.value)}
                  />
                </label>
              </div>

              <div className="mt-5">
                <p className="mb-2 text-sm font-medium">引用素材</p>
                <div className="flex flex-wrap gap-2">
                  {materials.map((material) => (
                    <button
                      key={material.id}
                      type="button"
                      className={cn(
                        'rounded-lg border px-3 py-2 text-sm transition-colors',
                        selectedMaterialIds.includes(material.id)
                          ? 'border-primary bg-primary text-primary-foreground'
                          : 'border-border bg-background text-muted-foreground hover:text-foreground',
                      )}
                      onClick={() => toggleMaterial(material.id)}
                    >
                      {material.materialName}
                    </button>
                  ))}
                </div>
              </div>

              <div className="mt-5 flex justify-end">
                <Button
                  type="submit"
                  disabled={createReportMutation.isPending || createJobMutation.isPending}
                >
                  {(createReportMutation.isPending || createJobMutation.isPending) && (
                    <Loader2 className="size-4 animate-spin" />
                  )}
                  创建草稿并生成大纲
                </Button>
              </div>
            </form>
          )}

          {step === 'outline' && (
            <section className="rounded-lg border border-border bg-card p-5">
              <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
                <div>
                  <h2 className="text-base font-semibold">大纲章节</h2>
                  <p className="mt-1 text-sm text-muted-foreground">
                    保存整棵章节树，后端负责合法性校验和重新编号。
                  </p>
                </div>
                <div className="flex gap-2">
                  <Button variant="outline" onClick={handleSaveOutline}>
                    <Save className="size-4" />
                    保存大纲
                  </Button>
                  <Button onClick={handleGenerateContent}>
                    <Play className="size-4" />
                    生成正文
                  </Button>
                </div>
              </div>
              <div className="space-y-2">
                {outline.map((node) => (
                  <div
                    key={node.id ?? node.clientSectionId ?? node.title}
                    className={cn(
                      'flex items-center gap-3 rounded-lg border border-border bg-background px-3 py-2',
                      node.level > 1 && 'ml-8',
                    )}
                  >
                    <span className="w-10 text-xs text-muted-foreground">
                      {node.numbering ?? '-'}
                    </span>
                    <span className="min-w-0 flex-1 truncate text-sm font-medium">
                      {node.title}
                    </span>
                    <span className="rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground">
                      level {node.level}
                    </span>
                  </div>
                ))}
              </div>
            </section>
          )}

          {step === 'content' && (
            <section className="grid gap-4 rounded-lg border border-border bg-card p-5 lg:grid-cols-[280px_minmax(0,1fr)]">
              <div>
                <h2 className="mb-3 text-base font-semibold">章节列表</h2>
                <div className="space-y-2">
                  {sections.map((section) => (
                    <button
                      key={section.id}
                      type="button"
                      className={cn(
                        'flex w-full items-center justify-between rounded-lg border px-3 py-2 text-left text-sm',
                        activeSection?.id === section.id
                          ? 'border-primary bg-primary/10 text-primary'
                          : 'border-border bg-background text-muted-foreground hover:text-foreground',
                      )}
                      onClick={() => setActiveSectionId(section.id)}
                    >
                      <span className="min-w-0 truncate">
                        {section.numbering} {section.title}
                      </span>
                      <span>{statusText[section.generationStatus]}</span>
                    </button>
                  ))}
                </div>
              </div>

              <div className="min-w-0">
                <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
                  <div>
                    <h3 className="text-base font-semibold">
                      {activeSection?.title ?? '章节正文'}
                    </h3>
                    <p className="text-sm text-muted-foreground">
                      保存章节只提交结构化正文，不直接生成 DOCX。
                    </p>
                  </div>
                  <div className="flex gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setShowVersions((prev) => !prev)}
                    >
                      版本记录{showVersions ? ' ▲' : ' ▼'}
                    </Button>
                    <Button
                      variant="outline"
                      onClick={handleRetry}
                      disabled={
                        effectiveJob?.status !== 'failed' &&
                        effectiveJob?.status !== 'partial_succeeded' &&
                        effectiveJob?.status !== 'canceled'
                      }
                    >
                      <RefreshCw className="size-4" />
                      重试任务
                    </Button>
                    <Button variant="outline" onClick={handleSaveSection}>
                      <PencilLine className="size-4" />
                      保存章节
                    </Button>
                    <Button onClick={handleExport}>
                      <Download className="size-4" />
                      创建 DOCX
                    </Button>
                  </div>
                </div>

                {showVersions && (
                  <div className="mb-4 rounded-lg border border-border bg-muted/30 p-3">
                    <h4 className="mb-2 text-sm font-medium">历史版本</h4>
                    {sectionVersionsQuery.isLoading ? (
                      <p className="text-xs text-muted-foreground">加载中...</p>
                    ) : sectionVersionsQuery.isError ? (
                      <p className="text-xs text-muted-foreground">
                        当前使用本地演示可用，真实接口联通后将列出章节版本记录。
                      </p>
                    ) : sectionVersionsQuery.data && sectionVersionsQuery.data.length > 0 ? (
                      <div className="max-h-40 space-y-2 overflow-auto">
                        {(sectionVersionsQuery.data as ReportSectionVersion[]).map((version) => (
                          <div
                            key={version.id}
                            className="flex items-center justify-between rounded-lg border border-border bg-background px-3 py-2 text-xs"
                          >
                            <div className="flex items-center gap-3">
                              <span className="font-medium">v{version.version}</span>
                              <span className="rounded-full bg-muted px-2 py-0.5 text-muted-foreground">
                                {version.source === 'manual' ? '手动' : 'AI'}
                              </span>
                              <span className="text-muted-foreground">
                                {formatDate(version.createdAt)}
                              </span>
                            </div>
                            {version.content && (
                              <button
                                type="button"
                                className="text-primary hover:underline"
                                onClick={() => {
                                  setSectionDraft(version.content ?? '')
                                  setNotice(`已恢复版本 v${version.version} 的内容到编辑区。`)
                                }}
                              >
                                恢复
                              </button>
                            )}
                          </div>
                        ))}
                      </div>
                    ) : (
                      <p className="text-xs text-muted-foreground">暂无历史版本。</p>
                    )}
                  </div>
                )}

                <textarea
                  className="min-h-[360px] w-full rounded-lg border border-input bg-background px-4 py-3 text-sm leading-7 outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
                  value={sectionDraft}
                  onChange={(event) => setSectionDraft(event.target.value)}
                />
              </div>
            </section>
          )}

          {step === 'export' && (
            <section className="rounded-lg border border-border bg-card p-5">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <h2 className="text-base font-semibold">DOCX 文件资源</h2>
                  <p className="mt-1 text-sm text-muted-foreground">
                    导出通过 POST /api/v1/report-files 创建资源；下载读取文件内容接口。
                  </p>
                </div>
                <Button onClick={handleDownload} disabled={!latestFile}>
                  <Download className="size-4" />
                  下载文件
                </Button>
              </div>

              <div className="mt-4 rounded-lg border border-border bg-background p-4">
                {latestFile ? (
                  <div className="grid gap-2 text-sm md:grid-cols-2">
                    <span className="text-muted-foreground">文件 ID</span>
                    <code>{latestFile.id}</code>
                    <span className="text-muted-foreground">文件名</span>
                    <span>{latestFile.filename ?? `${form.name}.docx`}</span>
                    <span className="text-muted-foreground">状态</span>
                    <span>{statusText[latestFile.status]}</span>
                    <span className="text-muted-foreground">创建时间</span>
                    <span>{formatDate(latestFile.createdAt)}</span>
                  </div>
                ) : (
                  <p className="text-sm text-muted-foreground">
                    尚未创建导出文件。请先生成正文后创建 DOCX 文件资源。
                  </p>
                )}
              </div>
            </section>
          )}
        </div>

        <aside className="space-y-4">
          <section className="rounded-lg border border-border bg-card p-4">
            <h2 className="text-sm font-semibold">当前报告</h2>
            <div className="mt-3 space-y-2 text-sm">
              <div className="flex justify-between gap-4">
                <span className="text-muted-foreground">reportId</span>
                <code className="truncate">{currentReport?.id ?? '-'}</code>
              </div>
              <div className="flex justify-between gap-4">
                <span className="text-muted-foreground">模板</span>
                <span className="truncate">{selectedTemplate?.templateName ?? '-'}</span>
              </div>
              <div className="flex justify-between gap-4">
                <span className="text-muted-foreground">状态</span>
                <span>{currentReport?.status ?? '未创建'}</span>
              </div>
            </div>
          </section>

          <section className="rounded-lg border border-border bg-card p-4">
            <h2 className="text-sm font-semibold">任务状态</h2>
            <div className="mt-3 space-y-3">
              <div className="flex justify-between gap-4 text-sm">
                <span className="text-muted-foreground">jobId</span>
                <code className="truncate">{effectiveJob?.id ?? '-'}</code>
              </div>
              <div className="flex justify-between gap-4 text-sm">
                <span className="text-muted-foreground">任务类型</span>
                <span>{effectiveJob?.jobType ?? '-'}</span>
              </div>
              <div className="flex justify-between gap-4 text-sm">
                <span className="text-muted-foreground">状态</span>
                <span
                  className={cn(
                    effectiveJob?.status === 'failed' && 'text-destructive',
                    effectiveJob?.status === 'canceled' && 'text-yellow-600',
                    effectiveJob?.status === 'succeeded' && 'text-green-600',
                    (effectiveJob?.status === 'running' || effectiveJob?.status === 'pending') &&
                      'text-primary',
                  )}
                >
                  {effectiveJob ? statusText[effectiveJob.status] : '-'}
                </span>
              </div>
              <div>
                <div className="mb-1 flex justify-between text-xs text-muted-foreground">
                  <span>进度</span>
                  <span>{progressPercent}%</span>
                </div>
                <div className="h-2 overflow-hidden rounded-full bg-muted">
                  <div
                    className={cn(
                      'h-full rounded-full transition-all',
                      effectiveJob?.status === 'canceled' ? 'bg-yellow-500' : 'bg-primary',
                      effectiveJob?.status === 'failed' && 'bg-destructive',
                    )}
                    style={{ width: `${progressPercent}%` }}
                  />
                </div>
              </div>
              {(effectiveJob?.status === 'pending' || effectiveJob?.status === 'running') && (
                <Button
                  variant="destructive"
                  size="sm"
                  className="w-full"
                  onClick={handleCancel}
                  title="任务取消暂不支持（Gateway 契约待补齐）"
                  disabled={
                    cancelJobMutation.isPending || Boolean(effectiveJob.id.startsWith('local-'))
                  }
                >
                  {cancelJobMutation.isPending && <Loader2 className="size-3 animate-spin" />}
                  <Ban className="size-3" />
                  取消任务
                </Button>
              )}
              {effectiveJob?.error?.message && (
                <p className="rounded-lg bg-destructive/10 p-3 text-sm text-destructive">
                  {effectiveJob.error.message}
                </p>
              )}
              {effectiveJob?.resultSummary && (
                <p className="rounded-lg bg-muted p-3 text-sm text-muted-foreground">
                  {effectiveJob.resultSummary}
                </p>
              )}
            </div>
          </section>

          {eventsQuery.data && eventsQuery.data.length > 0 && (
            <section className="rounded-lg border border-border bg-card p-4">
              <h2 className="text-sm font-semibold">事件日志</h2>
              <div className="mt-3 max-h-48 space-y-2 overflow-auto">
                {eventsQuery.data
                  .slice(-10)
                  .reverse()
                  .map((event) => (
                    <div
                      key={event.id}
                      className="rounded-lg border border-border bg-background px-3 py-2 text-xs"
                    >
                      <div className="flex justify-between text-muted-foreground">
                        <span className="font-medium">{event.eventType}</span>
                        <span>{formatDate(event.createdAt)}</span>
                      </div>
                      {event.message && <p className="mt-1 text-foreground">{event.message}</p>}
                    </div>
                  ))}
              </div>
            </section>
          )}

          <section className="rounded-lg border border-border bg-card p-4">
            <h2 className="text-sm font-semibold">接口契约</h2>
            <div className="mt-3 flex flex-wrap gap-2">
              {[
                'POST /api/v1/reports',
                'POST /api/v1/reports/{reportId}/jobs',
                'PATCH /api/v1/reports/{reportId}/outlines/{outlineId}',
                'PATCH /api/v1/reports/{reportId}/sections/{sectionId}',
                'POST /api/v1/report-files',
                'GET /api/v1/report-files/{reportFileId}/content',
              ].map((path) => (
                <span
                  key={path}
                  className="rounded-full border border-border bg-background px-2 py-1 text-xs text-muted-foreground"
                >
                  {path}
                </span>
              ))}
            </div>
          </section>
        </aside>
      </div>
    </div>
  )
}
