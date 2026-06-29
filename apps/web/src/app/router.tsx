import {
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  redirect,
  useNavigate,
  useSearch,
} from '@tanstack/react-router'

import { AppLayout } from '@/layouts/app-layout'
import type { PermissionRequirement } from '@/lib/permissions'
import { canAccess } from '@/lib/permissions'
import { KnowledgeConfig } from '@/pages/admin/knowledge-config'
import { KnowledgeManagement } from '@/pages/admin/knowledge-management'
import { ModelProfilesPage } from '@/pages/admin/model-profiles'
import { AdminPage } from '@/pages/admin/page'
import { ParserConfigsPage } from '@/pages/admin/parser-configs'
import { QARetrievalTestPage } from '@/pages/admin/qa-retrieval-test'
import { QASettings } from '@/pages/admin/qa-settings'
import { StatsOverviewPage } from '@/pages/admin/stats-overview'
import { StyleManagement } from '@/pages/admin/style-management'
import { SystemSettings } from '@/pages/admin/system-settings'
import { ForbiddenPage } from '@/pages/auth/forbidden'
import { KnowledgeChunksPage } from '@/pages/knowledge/chunks/page'
import { KnowledgeDocumentsPage } from '@/pages/knowledge/documents/page'
import { KnowledgeSearchPage } from '@/pages/knowledge/search/page'
import { LoginPage } from '@/pages/login/page'
import { ChatPage } from '@/pages/qa/chat/page'
import { ReportGeneratePage } from '@/pages/reports/generate/page'
import { ReportRecordsPage } from '@/pages/reports/records/page'
import { ReportTemplatesPage } from '@/pages/reports/templates/page'
import { useAuthStore } from '@/stores/auth-store'

async function restoreAuthForRoute() {
  const store = useAuthStore.getState()

  if (store.status === 'idle' || (store.accessToken && !store.user)) {
    await store.restoreSession()
  }

  return useAuthStore.getState()
}

function requireAuth(requirement?: PermissionRequirement) {
  return async () => {
    const store = await restoreAuthForRoute()

    if (store.status === 'anonymous') {
      throw redirect({ to: '/login' })
    }

    if (store.status === 'error') {
      return
    }

    if (!canAccess(store.user, requirement)) {
      throw redirect({ to: '/forbidden' })
    }
  }
}

async function redirectToReportHome() {
  const store = await restoreAuthForRoute()

  if (canAccess(store.user, reportWriteAccess)) {
    throw redirect({ to: '/reports/generate' })
  }

  if (canAccess(store.user, reportAccess)) {
    throw redirect({ to: '/reports/records' })
  }

  throw redirect({ to: '/forbidden' })
}

async function redirectToAppHome() {
  const store = await restoreAuthForRoute()

  if (canAccess(store.user, qaAccess)) {
    throw redirect({ to: '/chat' })
  }

  if (canAccess(store.user, reportWriteAccess)) {
    throw redirect({ to: '/reports/generate' })
  }

  if (canAccess(store.user, reportAccess)) {
    throw redirect({ to: '/reports/records' })
  }

  if (canAccess(store.user, adminAccess)) {
    throw redirect({ to: '/admin' })
  }

  throw redirect({ to: '/forbidden' })
}

async function redirectToAdminHome() {
  const store = await restoreAuthForRoute()

  if (canAccess(store.user, systemAdminAccess)) {
    throw redirect({ to: '/admin/stats' })
  }

  if (canAccess(store.user, reportAccess)) {
    throw redirect({ to: '/admin/reports/records' })
  }

  if (canAccess(store.user, knowledgeWriteAccess)) {
    throw redirect({ to: '/admin/knowledge' })
  }

  if (canAccess(store.user, knowledgeAccess)) {
    throw redirect({ to: '/admin/knowledge-config' })
  }

  if (canAccess(store.user, qaAdminAccess)) {
    throw redirect({ to: '/admin/qa-settings' })
  }

  throw redirect({ to: '/forbidden' })
}

const qaAccess: PermissionRequirement = { any: ['qa:use'] }
const qaAdminAccess: PermissionRequirement = {
  any: ['admin:model-profile:write', 'admin:parser-config:write', 'system:admin'],
}
const reportAccess: PermissionRequirement = {
  any: ['report:read', 'report:write', 'reports:write'],
}
const reportWriteAccess: PermissionRequirement = { any: ['report:write', 'reports:write'] }
const knowledgeAccess: PermissionRequirement = {
  any: ['knowledge:read', 'knowledge:write', 'document:upload'],
}
const knowledgeWriteAccess: PermissionRequirement = { any: ['knowledge:write'] }
const systemAdminAccess: PermissionRequirement = { any: ['system:admin'] }
const adminAccess: PermissionRequirement = {
  any: [
    'system:admin',
    'report:read',
    'report:write',
    'reports:write',
    'knowledge:read',
    'knowledge:write',
    'document:upload',
    'admin:model-profile:write',
    'admin:parser-config:write',
  ],
}

const rootRoute = createRootRoute({
  component: Outlet,
})

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'login',
  beforeLoad: async () => {
    const store = await restoreAuthForRoute()
    if (store.status === 'authenticated') {
      await redirectToAppHome()
    }
  },
  component: LoginPage,
})

const authenticatedRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: 'authenticated',
  beforeLoad: requireAuth(),
  component: () => (
    <AppLayout>
      <Outlet />
    </AppLayout>
  ),
})

const indexRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/',
  beforeLoad: redirectToAppHome,
})

const forbiddenRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: 'forbidden',
  component: ForbiddenPage,
})

const chatRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: 'chat',
  beforeLoad: requireAuth(qaAccess),
  component: ChatPage,
})

const reportsRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: 'reports',
  beforeLoad: requireAuth(reportAccess),
  component: Outlet,
})

const reportsIndexRoute = createRoute({
  getParentRoute: () => reportsRoute,
  path: '/',
  beforeLoad: redirectToReportHome,
})

const reportGenerateRoute = createRoute({
  getParentRoute: () => reportsRoute,
  path: 'generate',
  beforeLoad: requireAuth(reportWriteAccess),
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
  beforeLoad: requireAuth(reportWriteAccess),
  component: ReportTemplatesPage,
})

const adminRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: 'admin',
  beforeLoad: requireAuth(adminAccess),
  component: AdminPage,
})

const adminIndexRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: '/',
  beforeLoad: redirectToAdminHome,
})

const adminStylesRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'styles',
  beforeLoad: requireAuth(systemAdminAccess),
  component: StyleManagement,
})

const adminKnowledgeRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'knowledge',
  beforeLoad: requireAuth(knowledgeWriteAccess),
  component: KnowledgeManagement,
})

const adminKnowledgeConfigRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'knowledge-config',
  beforeLoad: requireAuth(knowledgeAccess),
  component: KnowledgeConfig,
})

// ── Knowledge sub-pages ──

interface AdminKnowledgeDocumentsSearch {
  knowledgeBaseId?: string
}

function AdminKnowledgeDocumentsPage() {
  const navigate = useNavigate()
  const search = useSearch({ strict: false }) as AdminKnowledgeDocumentsSearch

  return (
    <KnowledgeDocumentsPage
      knowledgeBaseId={search.knowledgeBaseId}
      onNavigateChunks={(documentId: string) => {
        void navigate({
          to: '/admin/knowledge/chunks',
          search: { documentId },
        })
      }}
    />
  )
}

const adminKnowledgeDocumentsRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'knowledge/documents',
  beforeLoad: requireAuth(knowledgeAccess),
  component: AdminKnowledgeDocumentsPage,
  validateSearch: (search: Record<string, unknown>): AdminKnowledgeDocumentsSearch => ({
    knowledgeBaseId:
      typeof search.knowledgeBaseId === 'string' ? search.knowledgeBaseId : undefined,
  }),
})

const adminKnowledgeSearchRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'knowledge/search',
  beforeLoad: requireAuth({ any: ['knowledge:read'] }),
  component: KnowledgeSearchPage,
})

interface AdminKnowledgeChunksSearch {
  documentId: string
}

function AdminKnowledgeChunksPage() {
  const navigate = useNavigate()
  const search = useSearch({ strict: false }) as AdminKnowledgeChunksSearch

  return (
    <KnowledgeChunksPage
      documentId={search.documentId}
      onNavigateBack={() => {
        void navigate({ to: '/admin/knowledge/documents' })
      }}
    />
  )
}

const adminKnowledgeChunksRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'knowledge/chunks',
  beforeLoad: requireAuth(knowledgeAccess),
  component: AdminKnowledgeChunksPage,
  validateSearch: (search: Record<string, unknown>): AdminKnowledgeChunksSearch => {
    const documentId = typeof search.documentId === 'string' ? search.documentId : ''
    return { documentId }
  },
})

const adminQASettingsRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'qa-settings',
  beforeLoad: requireAuth(qaAdminAccess),
  component: QASettings,
})

const adminQARetrievalTestRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'qa-retrieval-test',
  beforeLoad: requireAuth(qaAdminAccess),
  component: QARetrievalTestPage,
})

const adminSettingsRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'settings',
  beforeLoad: requireAuth(systemAdminAccess),
  component: SystemSettings,
})

const adminStatsRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'stats',
  beforeLoad: requireAuth(systemAdminAccess),
  component: StatsOverviewPage,
})

const modelProfilesPerm: PermissionRequirement = { any: ['admin:model-profile:write', 'system:admin'] }
const parserConfigsPerm: PermissionRequirement = { any: ['admin:parser-config:write', 'system:admin'] }

const adminModelProfilesRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'model-profiles',
  beforeLoad: requireAuth(modelProfilesPerm),
  component: ModelProfilesPage,
})

const adminParserConfigsRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'parser-configs',
  beforeLoad: requireAuth(parserConfigsPerm),
  component: ParserConfigsPage,
})

const adminReportRecordsRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'reports/records',
  beforeLoad: requireAuth(reportAccess),
  component: ReportRecordsPage,
})

const adminReportTemplatesRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'reports/templates',
  beforeLoad: requireAuth(reportWriteAccess),
  component: ReportTemplatesPage,
})

const routeTree = rootRoute.addChildren([
  loginRoute,
  authenticatedRoute.addChildren([
    indexRoute,
    forbiddenRoute,
    chatRoute,
    reportsRoute.addChildren([
      reportsIndexRoute,
      reportGenerateRoute,
      reportRecordsRoute,
      reportTemplatesRoute,
    ]),
    adminRoute.addChildren([
      adminIndexRoute,
      adminStylesRoute,
      adminKnowledgeRoute,
      adminKnowledgeConfigRoute,
      adminKnowledgeDocumentsRoute,
      adminKnowledgeSearchRoute,
      adminKnowledgeChunksRoute,
      adminQASettingsRoute,
      adminQARetrievalTestRoute,
      adminModelProfilesRoute,
      adminParserConfigsRoute,
      adminSettingsRoute,
      adminStatsRoute,
      adminReportRecordsRoute,
      adminReportTemplatesRoute,
    ]),
  ]),
])

export const router = createRouter({ routeTree })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
