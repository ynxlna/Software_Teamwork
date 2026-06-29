import { buildQuery, gatewayFileRequest, gatewayPageRequest, gatewayRequest } from '@/api/client'

import type {
  CreateReportJobPayload,
  CreateReportPayload,
  Report,
  ReportDailyStatistic,
  ReportEvent,
  ReportFile,
  ReportJob,
  ReportMaterial,
  ReportOutline,
  ReportSection,
  ReportSectionVersion,
  ReportStatisticsOverview,
  ReportStatus,
  ReportTemplate,
  ReportTemplateStructure,
  ReportType,
  ReportTypeCode,
} from './report-generation.types'

export type ReportListParams = {
  page?: number
  pageSize?: number
  reportType?: ReportTypeCode
  status?: ReportStatus | string
  keyword?: string
}

export type ReportTemplateListParams = {
  page?: number
  pageSize?: number
  reportType?: ReportTypeCode
  enabled?: boolean
}

export type ReportMaterialListParams = {
  page?: number
  pageSize?: number
  category?: string
  enabled?: boolean
}

export function listReportTypes(): Promise<ReportType[]> {
  return gatewayRequest<ReportType[]>('/report-types')
}

export function listReportTemplates(params: ReportTemplateListParams = {}) {
  return gatewayPageRequest<ReportTemplate>(`/report-templates${buildQuery(params)}`)
}

export function listReportMaterials(params: ReportMaterialListParams = {}) {
  return gatewayPageRequest<ReportMaterial>(`/report-materials${buildQuery(params)}`)
}

export function createReport(payload: CreateReportPayload): Promise<Report> {
  return gatewayRequest<Report>('/reports', {
    method: 'POST',
    body: payload,
  })
}

export function listReports(params: ReportListParams = {}) {
  return gatewayPageRequest<Report>(`/reports${buildQuery(params)}`)
}

export function listReportOutlines(reportId: string): Promise<ReportOutline[]> {
  return gatewayRequest<ReportOutline[]>(`/reports/${encodeURIComponent(reportId)}/outlines`)
}

export function updateReportOutline(
  reportId: string,
  outlineId: string,
  sections: ReportOutline['sections'],
): Promise<ReportOutline> {
  return gatewayRequest<ReportOutline>(
    `/reports/${encodeURIComponent(reportId)}/outlines/${encodeURIComponent(outlineId)}`,
    {
      method: 'PATCH',
      body: { sections, manualEdited: true },
    },
  )
}

export function listReportSections(reportId: string): Promise<ReportSection[]> {
  return gatewayRequest<ReportSection[]>(`/reports/${encodeURIComponent(reportId)}/sections`)
}

export function updateReportSection(
  reportId: string,
  sectionId: string,
  payload: { title?: string; content?: string; tables?: Record<string, unknown>[] },
): Promise<ReportSection> {
  return gatewayRequest<ReportSection>(
    `/reports/${encodeURIComponent(reportId)}/sections/${encodeURIComponent(sectionId)}`,
    {
      method: 'PATCH',
      body: { ...payload, manualEdited: true },
    },
  )
}

export function createReportJob(
  reportId: string,
  payload: CreateReportJobPayload,
): Promise<ReportJob> {
  return gatewayRequest<ReportJob>(`/reports/${encodeURIComponent(reportId)}/jobs`, {
    method: 'POST',
    body: payload,
  })
}

export function getReportJob(jobId: string): Promise<ReportJob> {
  return gatewayRequest<ReportJob>(`/report-jobs/${encodeURIComponent(jobId)}`)
}

export function createReportJobAttempt(jobId: string): Promise<ReportJob> {
  return gatewayRequest<ReportJob>(`/report-jobs/${encodeURIComponent(jobId)}/attempts`, {
    method: 'POST',
    body: { reason: 'frontend_retry' },
  })
}

export function createReportFile(payload: {
  reportId: string
  format: 'docx'
  templateId?: string
  styleOptions?: Record<string, unknown>
}): Promise<ReportFile> {
  return gatewayRequest<ReportFile>('/report-files', {
    method: 'POST',
    body: payload,
  })
}

export function downloadReportFile(reportFileId: string): Promise<Blob> {
  return gatewayFileRequest(`/report-files/${encodeURIComponent(reportFileId)}/content`)
}

export function getReportStatisticsOverview(): Promise<ReportStatisticsOverview> {
  return gatewayRequest<ReportStatisticsOverview>('/report-statistics/overview')
}

export function listDailyReportStatistics(days = 30): Promise<ReportDailyStatistic[]> {
  return gatewayRequest<ReportDailyStatistic[]>(`/report-statistics/daily${buildQuery({ days })}`)
}

export function getReport(reportId: string): Promise<Report> {
  return gatewayRequest<Report>(`/reports/${encodeURIComponent(reportId)}`)
}

export function deleteReport(reportId: string): Promise<void> {
  return gatewayRequest<void>(`/reports/${encodeURIComponent(reportId)}`, {
    method: 'DELETE',
  })
}

export function getReportTemplateStructure(templateId: string): Promise<ReportTemplateStructure> {
  return gatewayRequest<ReportTemplateStructure>(
    `/report-templates/${encodeURIComponent(templateId)}/structure`,
  )
}

export function updateReportTemplateStructure(
  templateId: string,
  payload: ReportTemplateStructure,
): Promise<ReportTemplateStructure> {
  return gatewayRequest<ReportTemplateStructure>(
    `/report-templates/${encodeURIComponent(templateId)}/structure`,
    {
      method: 'PATCH',
      body: payload,
    },
  )
}

export function deleteReportTemplate(templateId: string): Promise<void> {
  return gatewayRequest<void>(`/report-templates/${encodeURIComponent(templateId)}`, {
    method: 'DELETE',
  })
}

export function listReportEvents(reportId: string): Promise<ReportEvent[]> {
  return gatewayRequest<ReportEvent[]>(`/reports/${encodeURIComponent(reportId)}/events`)
}

/**
 * Cancel a running report job.
 * NOT YET SUPPORTED: the Gateway OpenAPI does not currently expose PATCH
 * for /api/v1/report-jobs/{jobId}. Will throw until contract is updated.
 */
export function cancelReportJob(_jobId: string): Promise<ReportJob> {
  throw new Error('Job cancellation is not yet supported by the Gateway contract.')
}

export function listSectionVersions(
  reportId: string,
  sectionId: string,
): Promise<ReportSectionVersion[]> {
  return gatewayRequest<ReportSectionVersion[]>(
    `/reports/${encodeURIComponent(reportId)}/sections/${encodeURIComponent(sectionId)}/versions`,
  )
}
