import { useQuery } from '@tanstack/react-query'
import { Database, Info } from 'lucide-react'

import { getCurrentQAConfig, listKnowledgeBases } from '@/api/admin'
import { Badge } from '@/components/ui/badge'

function KnowledgeConfigSkeleton() {
  return (
    <div className="animate-pulse space-y-6">
      {/* KB list skeleton */}
      <div className="rounded-lg border border-border bg-card p-6">
        <div className="mb-4 h-5 w-24 rounded bg-muted" />
        <div className="space-y-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <div
              key={i}
              className="flex items-center justify-between rounded-md border border-border p-3"
            >
              <div className="space-y-1">
                <div className="h-4 w-32 rounded bg-muted" />
                <div className="h-3 w-20 rounded bg-muted" />
              </div>
              <div className="h-5 w-14 rounded bg-muted" />
            </div>
          ))}
        </div>
      </div>

      {/* Defaults skeleton */}
      <div className="rounded-lg border border-border bg-card p-6">
        <div className="mb-4 h-5 w-32 rounded bg-muted" />
        <div className="grid grid-cols-2 gap-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="flex justify-between">
              <div className="h-4 w-20 rounded bg-muted" />
              <div className="h-4 w-16 rounded bg-muted" />
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

export function KnowledgeConfig() {
  const {
    data: knowledgeBases,
    isLoading: kbLoading,
    isError: kbError,
    error: kbErr,
  } = useQuery({
    queryKey: ['admin', 'knowledge-bases'],
    queryFn: listKnowledgeBases,
    staleTime: 60_000,
  })

  const {
    data: qaConfig,
    isLoading: configLoading,
    isError: configError,
    error: configErr,
  } = useQuery({
    queryKey: ['admin', 'qa-config'],
    queryFn: getCurrentQAConfig,
    staleTime: 60_000,
  })

  const isLoading = kbLoading || configLoading
  const isError = kbError || configError
  const errorMessage =
    kbErr instanceof Error
      ? kbErr.message
      : configErr instanceof Error
        ? configErr.message
        : '未知错误'

  return (
    <div>
      <h3 className="mb-4 text-2xl font-semibold text-foreground">知识配置</h3>
      <p className="mb-6 text-sm text-muted-foreground">查看知识库配置信息及默认 RAG 检索参数。</p>

      {/* Error state */}
      {isError && !isLoading && (
        <div className="mb-6 rounded-lg border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive">
          加载知识配置失败: {errorMessage}
        </div>
      )}

      {/* Loading state */}
      {isLoading && <KnowledgeConfigSkeleton />}

      {/* Data */}
      {!isLoading && !isError && (
        <div className="space-y-6">
          {/* Knowledge Bases list */}
          <div className="rounded-lg border border-border bg-card p-6">
            <h4 className="mb-4 flex items-center gap-2 text-lg font-semibold text-foreground">
              <Database aria-hidden="true" className="size-5" />
              知识库列表
            </h4>

            {!knowledgeBases || knowledgeBases.items.length === 0 ? (
              <div className="rounded-lg border border-dashed border-border p-6 text-center text-sm text-muted-foreground">
                暂无知识库
              </div>
            ) : (
              <div className="space-y-2">
                {knowledgeBases.items.map((kb) => (
                  <div
                    key={kb.id}
                    className="flex items-center justify-between rounded-md border border-border bg-background p-3"
                  >
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium text-foreground">{kb.name}</p>
                      <p className="text-xs text-muted-foreground">文档数: {kb.documentCount}</p>
                    </div>
                    <Badge variant="default">活跃</Badge>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* RAG Defaults */}
          {qaConfig && (
            <div className="rounded-lg border border-border bg-card p-6">
              <h4 className="mb-4 flex items-center gap-2 text-lg font-semibold text-foreground">
                <Info aria-hidden="true" className="size-5" />
                默认 RAG 参数
              </h4>

              <div className="grid grid-cols-2 gap-x-8 gap-y-3">
                <div className="flex justify-between border-b border-border pb-2">
                  <span className="text-sm text-muted-foreground">top_k</span>
                  <span className="text-sm font-medium text-foreground">
                    {qaConfig.retrieval?.topK ?? '-'}
                  </span>
                </div>
                <div className="flex justify-between border-b border-border pb-2">
                  <span className="text-sm text-muted-foreground">score_threshold</span>
                  <span className="text-sm font-medium text-foreground">
                    {qaConfig.retrieval?.scoreThreshold ?? '-'}
                  </span>
                </div>
                <div className="flex justify-between border-b border-border pb-2">
                  <span className="text-sm text-muted-foreground">enable_rerank</span>
                  <Badge variant={qaConfig.retrieval?.enableRerank ? 'default' : 'secondary'}>
                    {qaConfig.retrieval?.enableRerank ? '是' : '否'}
                  </Badge>
                </div>
                <div className="flex justify-between border-b border-border pb-2">
                  <span className="text-sm text-muted-foreground">rerank_threshold</span>
                  <span className="text-sm font-medium text-foreground">
                    {qaConfig.retrieval?.rerankThreshold ?? '-'}
                  </span>
                </div>
                <div className="flex justify-between border-b border-border pb-2">
                  <span className="text-sm text-muted-foreground">rerank_top_n</span>
                  <span className="text-sm font-medium text-foreground">
                    {qaConfig.retrieval?.rerankTopN ?? '-'}
                  </span>
                </div>
                <div className="flex justify-between border-b border-border pb-2">
                  <span className="text-sm text-muted-foreground">默认知识库</span>
                  <span className="text-sm font-medium text-foreground">
                    {qaConfig.defaultKnowledgeBaseIds &&
                    qaConfig.defaultKnowledgeBaseIds.length > 0
                      ? qaConfig.defaultKnowledgeBaseIds.join(', ')
                      : '无'}
                  </span>
                </div>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Empty state */}
      {!isLoading && !isError && !knowledgeBases && !qaConfig && (
        <div className="rounded-lg border border-dashed border-border p-10 text-center text-sm text-muted-foreground">
          暂无配置数据
        </div>
      )}
    </div>
  )
}
