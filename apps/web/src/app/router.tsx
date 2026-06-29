import {
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  redirect,
} from '@tanstack/react-router'

import { AppLayout } from '@/layouts/app-layout'
import { FileManagement } from '@/pages/admin/file-management'
import { KnowledgeConfig } from '@/pages/admin/knowledge-config'
import { KnowledgeExperience } from '@/pages/admin/knowledge-experience'
import { KnowledgeManagement } from '@/pages/admin/knowledge-management'
import { MaterialManagement } from '@/pages/admin/material-management'
import { AdminPage } from '@/pages/admin/page'
import { PromptManagement } from '@/pages/admin/prompt-management'
import { ReportCategory } from '@/pages/admin/report-category'
import { RoleManagement } from '@/pages/admin/role-management'
import { StatsOverviewPage } from '@/pages/admin/stats-overview'
import { StyleManagement } from '@/pages/admin/style-management'
import { SystemSettings } from '@/pages/admin/system-settings'
import { TemplateManagement } from '@/pages/admin/template-management'
import { UserManagement } from '@/pages/admin/user-management'
import { ChatPage } from '@/pages/qa/chat/page'
import { ReportGeneratePage } from '@/pages/reports/generate/page'
import { ReportRecordsPage } from '@/pages/reports/records/page'
import { ReportTemplatesPage } from '@/pages/reports/templates/page'

// ── Root route ──────────────────────────────────────────────
const rootRoute = createRootRoute({
  component: () => (
    <AppLayout>
      <Outlet />
    </AppLayout>
  ),
})

// ── Child routes ────────────────────────────────────────────

// Index: redirect to /chat
const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  beforeLoad: () => {
    throw redirect({ to: '/chat' })
  },
})

// Chat
const chatRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'chat',
  component: ChatPage,
})

// Reports
const reportsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'reports',
  component: Outlet,
})

const reportsIndexRoute = createRoute({
  getParentRoute: () => reportsRoute,
  path: '/',
  beforeLoad: () => {
    throw redirect({ to: '/reports/generate' })
  },
})

const reportGenerateRoute = createRoute({
  getParentRoute: () => reportsRoute,
  path: 'generate',
  component: ReportGeneratePage,
})

const reportRecordsRoute = createRoute({
  getParentRoute: () => reportsRoute,
  path: 'records',
  component: ReportRecordsPage,
})

const reportTemplatesRoute = createRoute({
  getParentRoute: () => reportsRoute,
  path: 'templates',
  component: ReportTemplatesPage,
})

// ── Admin layout route ──────────────────────────────────────
const adminRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'admin',
  component: AdminPage,
})

// Admin index → stats overview
const adminIndexRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: '/',
  component: StatsOverviewPage,
})

// Admin child routes
const adminUsersRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'users',
  component: UserManagement,
})

const adminRolesRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'roles',
  component: RoleManagement,
})

const adminStylesRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'styles',
  component: StyleManagement,
})

const adminReportCategoriesRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'report-categories',
  component: ReportCategory,
})

const adminFilesRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'files',
  component: FileManagement,
})

const adminTemplatesRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'templates',
  component: TemplateManagement,
})

const adminMaterialsRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'materials',
  component: MaterialManagement,
})

const adminPromptsRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'prompts',
  component: PromptManagement,
})

const adminKnowledgeRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'knowledge',
  component: KnowledgeManagement,
})

const adminKnowledgeExperienceRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'knowledge-experience',
  component: KnowledgeExperience,
})

const adminKnowledgeConfigRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'knowledge-config',
  component: KnowledgeConfig,
})

const adminSettingsRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'settings',
  component: SystemSettings,
})

const adminStatsRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'stats',
  component: StatsOverviewPage,
})

const adminReportRecordsRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'reports/records',
  component: ReportRecordsPage,
})

const adminReportTemplatesRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'reports/templates',
  component: ReportTemplatesPage,
})

// ── Route tree ──────────────────────────────────────────────
const routeTree = rootRoute.addChildren([
  indexRoute,
  chatRoute,
  reportsRoute.addChildren([
    reportsIndexRoute,
    reportGenerateRoute,
    reportRecordsRoute,
    reportTemplatesRoute,
  ]),
  adminRoute.addChildren([
    adminIndexRoute,
    adminUsersRoute,
    adminRolesRoute,
    adminStylesRoute,
    adminReportCategoriesRoute,
    adminFilesRoute,
    adminTemplatesRoute,
    adminMaterialsRoute,
    adminPromptsRoute,
    adminKnowledgeRoute,
    adminKnowledgeExperienceRoute,
    adminKnowledgeConfigRoute,
    adminSettingsRoute,
    adminStatsRoute,
    adminReportRecordsRoute,
    adminReportTemplatesRoute,
  ]),
])

// ── Router instance ─────────────────────────────────────────
export const router = createRouter({ routeTree })

// ── Type registration ───────────────────────────────────────
declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
