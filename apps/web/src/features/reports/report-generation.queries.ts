import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import {
  cancelReportJob,
  createReport,
  createReportFile,
  createReportJob,
  createReportJobAttempt,
  deleteReport,
  deleteReportTemplate,
  downloadReportFile,
  getReport,
  getReportJob,
  getReportStatisticsOverview,
  getReportTemplateStructure,
  listDailyReportStatistics,
  listReportEvents,
  listReportMaterials,
  listReportOutlines,
  listReports,
  listReportSections,
  listReportTemplates,
  listReportTypes,
  listSectionVersions,
  updateReportOutline,
  updateReportSection,
  updateReportTemplateStructure,
} from './report-generation.api'
import type {
  CreateReportJobPayload,
  CreateReportPayload,
  ReportOutline,
  ReportTemplateStructure,
} from './report-generation.types'

export const reportKeys = {
  all: ['reports'] as const,
  types: () => [...reportKeys.all, 'types'] as const,
  templates: () => [...reportKeys.all, 'templates'] as const,
  materials: () => [...reportKeys.all, 'materials'] as const,
  records: () => [...reportKeys.all, 'records'] as const,
  recordList: (keyword: string) => [...reportKeys.records(), { keyword }] as const,
  detail: (reportId: string) => [...reportKeys.all, 'detail', reportId] as const,
  outlines: (reportId: string) => [...reportKeys.all, reportId, 'outlines'] as const,
  sections: (reportId: string) => [...reportKeys.all, reportId, 'sections'] as const,
  job: (jobId: string) => [...reportKeys.all, 'jobs', jobId] as const,
  events: (reportId: string) => [...reportKeys.all, reportId, 'events'] as const,
  sectionVersions: (reportId: string, sectionId: string) =>
    [...reportKeys.all, reportId, 'sections', sectionId, 'versions'] as const,
  stats: () => [...reportKeys.all, 'statistics'] as const,
  templateStructure: (templateId: string) =>
    [...reportKeys.templates(), templateId, 'structure'] as const,
}

export function useReportBootstrapQueries(reportType?: string) {
  const typeQuery = useQuery({
    queryKey: reportKeys.types(),
    queryFn: listReportTypes,
  })
  const templateQuery = useQuery({
    queryKey: [...reportKeys.templates(), { reportType }],
    queryFn: () =>
      listReportTemplates({
        reportType,
        enabled: true,
        page: 1,
        pageSize: 20,
      }),
  })
  const materialQuery = useQuery({
    queryKey: reportKeys.materials(),
    queryFn: () => listReportMaterials({ enabled: true, page: 1, pageSize: 20 }),
  })

  return { typeQuery, templateQuery, materialQuery }
}

export function useReportsQuery(keyword = '') {
  return useQuery({
    queryKey: reportKeys.recordList(keyword),
    queryFn: () => listReports({ keyword, page: 1, pageSize: 20 }),
  })
}

export function useReportDetailQueries(reportId: string | null) {
  const enabled = Boolean(reportId)
  const outlinesQuery = useQuery({
    queryKey: reportKeys.outlines(reportId ?? ''),
    queryFn: () => listReportOutlines(reportId ?? ''),
    enabled,
  })
  const sectionsQuery = useQuery({
    queryKey: reportKeys.sections(reportId ?? ''),
    queryFn: () => listReportSections(reportId ?? ''),
    enabled,
  })

  return { outlinesQuery, sectionsQuery }
}

export function useReportJobQuery(jobId: string | null) {
  return useQuery({
    queryKey: reportKeys.job(jobId ?? ''),
    queryFn: () => getReportJob(jobId ?? ''),
    enabled: Boolean(jobId),
    refetchInterval: (query) => {
      const status = query.state.data?.status
      return status === 'pending' || status === 'running' ? 3000 : false
    },
  })
}

export function useCreateReportMutation() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (payload: CreateReportPayload) => createReport(payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: reportKeys.records() })
    },
  })
}

export function useCreateReportJobMutation() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ reportId, payload }: { reportId: string; payload: CreateReportJobPayload }) =>
      createReportJob(reportId, payload),
    onSuccess: (job) => {
      void queryClient.invalidateQueries({
        queryKey: reportKeys.outlines(job.reportId),
      })
      void queryClient.invalidateQueries({
        queryKey: reportKeys.sections(job.reportId),
      })
      void queryClient.invalidateQueries({ queryKey: reportKeys.records() })
    },
  })
}

export function useUpdateReportOutlineMutation(reportId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({
      outlineId,
      sections,
    }: {
      outlineId: string
      sections: ReportOutline['sections']
    }) => updateReportOutline(reportId, outlineId, sections),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: reportKeys.outlines(reportId),
      })
    },
  })
}

export function useUpdateReportSectionMutation(reportId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({
      sectionId,
      title,
      content,
    }: {
      sectionId: string
      title?: string
      content?: string
    }) => updateReportSection(reportId, sectionId, { title, content }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: reportKeys.sections(reportId),
      })
    },
  })
}

export function useCreateReportFileMutation() {
  return useMutation({
    mutationFn: createReportFile,
  })
}

export function useRetryReportJobMutation() {
  return useMutation({
    mutationFn: (jobId: string) => createReportJobAttempt(jobId),
  })
}

export function useDownloadReportFileMutation() {
  return useMutation({
    mutationFn: (reportFileId: string) => downloadReportFile(reportFileId),
  })
}

export function useReportStatisticsQueries() {
  const overviewQuery = useQuery({
    queryKey: reportKeys.stats(),
    queryFn: getReportStatisticsOverview,
  })
  const dailyQuery = useQuery({
    queryKey: [...reportKeys.stats(), 'daily'],
    queryFn: () => listDailyReportStatistics(30),
  })

  return { overviewQuery, dailyQuery }
}

export function useReport(reportId: string | null) {
  return useQuery({
    queryKey: reportKeys.detail(reportId ?? ''),
    queryFn: () => getReport(reportId ?? ''),
    enabled: Boolean(reportId),
  })
}

export function useDeleteReport() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (reportId: string) => deleteReport(reportId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: reportKeys.records() })
    },
  })
}

export function useTemplateStructure(templateId: string | null) {
  return useQuery({
    queryKey: reportKeys.templateStructure(templateId ?? ''),
    queryFn: () => getReportTemplateStructure(templateId ?? ''),
    enabled: Boolean(templateId),
  })
}

export function useUpdateTemplateStructure(templateId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (payload: ReportTemplateStructure) =>
      updateReportTemplateStructure(templateId, payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: reportKeys.templateStructure(templateId),
      })
    },
  })
}

export function useDeleteTemplate() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (templateId: string) => deleteReportTemplate(templateId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: reportKeys.templates() })
    },
  })
}

export function useReportEvents(reportId: string | null) {
  return useQuery({
    queryKey: reportKeys.events(reportId ?? ''),
    queryFn: () => listReportEvents(reportId ?? ''),
    enabled: Boolean(reportId),
    refetchInterval: (query) => {
      const events = query.state.data
      // Keep polling until a terminal event is received; empty list is normal
      // at job start and should not stop polling.
      if (!events || events.length === 0) return 5000
      const latest = events[events.length - 1]
      if (latest?.eventType === 'job.completed' || latest?.eventType === 'job.failed') {
        return false
      }
      return 5000
    },
    select: (data) => data,
  })
}

export function useCancelReportJob() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (jobId: string) => cancelReportJob(jobId),
    onSuccess: (job) => {
      void queryClient.invalidateQueries({ queryKey: reportKeys.job(job.id) })
      if (job.reportId) {
        void queryClient.invalidateQueries({
          queryKey: reportKeys.events(job.reportId),
        })
      }
    },
  })
}

export function useSectionVersions(reportId: string | null, sectionId: string | null) {
  return useQuery({
    queryKey: reportKeys.sectionVersions(reportId ?? '', sectionId ?? ''),
    queryFn: () => listSectionVersions(reportId ?? '', sectionId ?? ''),
    enabled: Boolean(reportId) && Boolean(sectionId),
  })
}
