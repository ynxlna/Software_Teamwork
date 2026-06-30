import { Link, useRouterState } from '@tanstack/react-router'
import {
  BarChart3,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Database,
  FileText,
  LayoutTemplate,
  MessageSquareText,
  Package,
  Settings,
  Wrench,
} from 'lucide-react'
import { useMemo, useState } from 'react'

import type { PermissionRequirement } from '@/lib/permissions'
import { canAccess } from '@/lib/permissions'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'
import { useUiStore } from '@/stores/ui-store'

type AdminNavigationItem = {
  children?: AdminNavigationItem[]
  key: string
  label: string
  path?: string
  requirement?: PermissionRequirement
}

const menuItems: AdminNavigationItem[] = [
  {
    key: 'system',
    label: '系统管理',
    requirement: { any: ['system:admin'] },
    children: [{ key: 'styles', label: '样式管理', path: '/admin/styles' }],
  },
  {
    key: 'reports',
    label: '报告生成',
    requirement: { any: ['report:read', 'report:write', 'reports:write'] },
    children: [
      { key: 'report-records', label: '报告记录', path: '/admin/reports/records' },
      {
        key: 'report-templates',
        label: '模板素材',
        path: '/admin/reports/templates',
        requirement: { any: ['report:write', 'reports:write'] },
      },
    ],
  },
  {
    key: 'rag',
    label: 'RAG 知识库',
    requirement: {
      any: [
        'system:admin',
        'knowledge:read',
        'knowledge:write',
        'document:upload',
        'admin:model-profile:write',
        'admin:parser-config:write',
      ],
    },
    children: [
      {
        key: 'knowledge',
        label: '知识管理',
        path: '/admin/knowledge',
        requirement: { any: ['knowledge:write'] },
      },
      {
        key: 'knowledge-documents',
        label: '文档管理',
        path: '/admin/knowledge/documents',
        requirement: { any: ['knowledge:read', 'knowledge:write', 'document:upload'] },
      },
      {
        key: 'knowledge-search',
        label: '知识检索',
        path: '/admin/knowledge/search',
        requirement: { any: ['knowledge:read'] },
      },
      {
        key: 'knowledge-config',
        label: '知识配置',
        path: '/admin/knowledge-config',
        requirement: { any: ['knowledge:read'] },
      },
      {
        key: 'parser-configs',
        label: '解析器配置',
        path: '/admin/parser-configs',
        requirement: { any: ['admin:parser-config:write', 'system:admin'] },
      },
      {
        key: 'qa-settings',
        label: 'QA / LLM 配置',
        path: '/admin/qa-settings',
        requirement: {
          any: ['admin:model-profile:write', 'admin:parser-config:write', 'system:admin'],
        },
      },
      {
        key: 'qa-retrieval-test',
        label: 'QA 检索测试',
        path: '/admin/qa-retrieval-test',
        requirement: {
          any: ['admin:model-profile:write', 'admin:parser-config:write', 'system:admin'],
        },
      },
      {
        key: 'model-profiles',
        label: '模型管理',
        path: '/admin/model-profiles',
        requirement: { any: ['admin:model-profile:write', 'system:admin'] },
      },
      {
        key: 'parser-configs',
        label: '解析器配置',
        path: '/admin/parser-configs',
        requirement: { any: ['admin:parser-config:write', 'system:admin'] },
      },
    ],
  },
  {
    key: 'stats',
    label: 'QA 统计',
    path: '/admin/stats',
    requirement: { any: ['system:admin'] },
  },
  {
    key: 'settings',
    label: '系统设置',
    path: '/admin/settings',
    requirement: { any: ['system:admin'] },
  },
]

const ICON_MAP: Record<string, typeof Settings> = {
  system: Settings,
  stats: BarChart3,
  reports: FileText,
  templates: LayoutTemplate,
  materials: Package,
  prompts: MessageSquareText,
  rag: Database,
  settings: Wrench,
}

function filterMenu(
  items: readonly AdminNavigationItem[],
  user: ReturnType<typeof useAuthStore.getState>['user'],
): AdminNavigationItem[] {
  return items
    .map((item) => {
      const children = item.children ? filterMenu(item.children, user) : undefined
      return { ...item, children }
    })
    .filter(
      (item) => canAccess(user, item.requirement) && (!item.children || item.children.length > 0),
    )
}

export function AdminSidebar() {
  const routerState = useRouterState()
  const pathname = routerState.location.pathname
  const user = useAuthStore((state) => state.user)
  const visibleMenuItems = useMemo(() => filterMenu(menuItems, user), [user])
  const [expanded, setExpanded] = useState<Set<string>>(new Set(['system', 'reports', 'rag']))
  const sidebarCollapsed = useUiStore((s) => s.sidebarCollapsed)
  const toggleSidebar = useUiStore((s) => s.toggleSidebar)

  const toggle = (key: string) => {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(key)) {
        next.delete(key)
      } else {
        next.add(key)
      }
      return next
    })
  }

  const handleGroupClick = (key: string) => {
    if (sidebarCollapsed) {
      toggleSidebar()
      setExpanded((prev) => {
        const next = new Set(prev)
        next.add(key)
        return next
      })
      return
    }

    toggle(key)
  }

  const isActive = (path?: string): boolean => {
    if (!path) return false
    return pathname === path || pathname.startsWith(`${path}/`)
  }

  return (
    <aside
      className={cn(
        'flex flex-shrink-0 flex-col overflow-hidden border-r border-border bg-sidebar',
        'transition-[width] duration-300',
        sidebarCollapsed ? 'w-14' : 'w-56',
      )}
    >
      <div className="flex items-center border-b border-border">
        {!sidebarCollapsed && (
          <h2 className="flex-1 whitespace-nowrap px-4 py-3 text-sm font-semibold text-sidebar-foreground">
            管理面板
          </h2>
        )}
        <button
          aria-label={sidebarCollapsed ? '展开侧边栏' : '折叠侧边栏'}
          className={cn(
            'flex shrink-0 items-center justify-center text-muted-foreground transition-all hover:bg-accent hover:text-foreground',
            sidebarCollapsed ? 'mx-auto my-3 size-7 rounded-md' : 'mr-1 size-7 rounded-md',
          )}
          type="button"
          onClick={toggleSidebar}
        >
          {sidebarCollapsed ? (
            <ChevronRight className="size-4 transition-transform duration-300" />
          ) : (
            <ChevronLeft className="size-4 transition-transform duration-300" />
          )}
        </button>
      </div>

      <nav className="flex flex-1 flex-col gap-0.5 overflow-auto py-1">
        {visibleMenuItems.map((item) => {
          const hasChildren = item.children && item.children.length > 0
          const Icon = ICON_MAP[item.key]

          if (hasChildren) {
            const open = expanded.has(item.key) && !sidebarCollapsed
            return (
              <div key={item.key}>
                <button
                  className={cn(
                    'flex w-full items-center text-left text-sm font-medium text-sidebar-foreground transition-colors hover:bg-primary/5 hover:text-primary',
                    sidebarCollapsed ? 'justify-center px-0 py-2' : 'gap-1.5 px-4 py-2',
                  )}
                  title={sidebarCollapsed ? item.label : undefined}
                  type="button"
                  onClick={() => handleGroupClick(item.key)}
                >
                  {sidebarCollapsed ? (
                    Icon && <Icon className="size-5 shrink-0" />
                  ) : (
                    <>
                      {open ? (
                        <ChevronDown
                          aria-hidden="true"
                          className="shrink-0 text-muted-foreground"
                          size={12}
                        />
                      ) : (
                        <ChevronRight
                          aria-hidden="true"
                          className="shrink-0 text-muted-foreground"
                          size={12}
                        />
                      )}
                      <span className="inline-block h-1.5 w-1.5 shrink-0 rounded-full bg-primary" />
                      <span className="whitespace-nowrap">{item.label}</span>
                    </>
                  )}
                </button>
                {open && (
                  <div className="bg-sidebar-accent/40 py-0.5 transition-[max-height] duration-200">
                    {item.children!.map((child) => (
                      <Link
                        key={child.key}
                        className={cn(
                          'block whitespace-nowrap px-4 py-1.5 pl-10 text-sm text-muted-foreground transition-colors hover:bg-primary/5 hover:text-primary',
                          isActive(child.path) && 'bg-primary/10 font-medium text-primary',
                        )}
                        to={child.path!}
                      >
                        {child.label}
                      </Link>
                    ))}
                  </div>
                )}
              </div>
            )
          }

          return (
            <Link
              key={item.key}
              className={cn(
                'flex items-center text-sm font-medium text-sidebar-foreground transition-colors hover:bg-primary/5 hover:text-primary',
                sidebarCollapsed ? 'justify-center px-0 py-2' : 'px-4 py-2',
                isActive(item.path) && 'bg-primary/10 font-medium text-primary',
              )}
              title={sidebarCollapsed ? item.label : undefined}
              to={item.path!}
            >
              {sidebarCollapsed && Icon ? (
                <Icon className="size-5 shrink-0" />
              ) : (
                <span className="whitespace-nowrap">{item.label}</span>
              )}
            </Link>
          )
        })}
      </nav>
    </aside>
  )
}
