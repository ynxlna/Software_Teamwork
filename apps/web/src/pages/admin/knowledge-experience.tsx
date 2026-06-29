import { useMutation } from '@tanstack/react-query'
import { Loader2, Search } from 'lucide-react'
import { useState } from 'react'

import { queryKnowledge } from '@/api/chat'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { KnowledgeQueryResult } from '@/lib/types'

export function KnowledgeExperience() {
  const [query, setQuery] = useState('')

  const searchMutation = useMutation({
    mutationFn: (searchQuery: string) => queryKnowledge({ query: searchQuery }),
  })

  const handleSearch = () => {
    const trimmed = query.trim()
    if (!trimmed || searchMutation.isPending) return
    searchMutation.mutate(trimmed)
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      handleSearch()
    }
  }

  const results: KnowledgeQueryResult[] = searchMutation.data?.results ?? []
  const totalHits = searchMutation.data?.trace.hitCount ?? 0

  return (
    <div>
      <h3 className="mb-4 text-2xl font-semibold text-foreground">知识体验</h3>
      <p className="mb-6 text-sm text-muted-foreground">
        以对话形式测试知识库的检索效果，验证检索结果的相关性和准确性。
      </p>

      {/* Search area */}
      <div className="rounded-lg border border-border bg-card p-6">
        <label htmlFor="rag-test-query" className="mb-2 block text-sm font-medium text-foreground">
          检索测试
        </label>
        <div className="flex gap-2">
          <Input
            id="rag-test-query"
            type="text"
            placeholder="输入测试问题…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={handleKeyDown}
            className="flex-1"
          />
          <Button
            onClick={handleSearch}
            disabled={searchMutation.isPending || query.trim().length === 0}
          >
            {searchMutation.isPending ? (
              <Loader2 aria-hidden="true" className="mr-1.5 size-3.5 animate-spin" />
            ) : (
              <Search aria-hidden="true" className="mr-1.5 size-3.5" />
            )}
            检索
          </Button>
        </div>

        {/* Error state */}
        {searchMutation.isError && (
          <div className="mt-4 rounded-lg border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive">
            检索失败:{' '}
            {searchMutation.error instanceof Error ? searchMutation.error.message : '未知错误'}
          </div>
        )}

        {/* Results */}
        {searchMutation.isSuccess && (
          <div className="mt-4">
            {/* Summary */}
            <div className="mb-3 flex items-center gap-3 text-sm text-muted-foreground">
              <span>
                共找到 <span className="font-semibold text-foreground">{totalHits}</span> 条结果
              </span>
            </div>

            {/* Result list */}
            {results.length === 0 ? (
              <div className="rounded-lg border border-dashed border-border p-10 text-center text-sm text-muted-foreground">
                未找到相关结果，请尝试其他关键词。
              </div>
            ) : (
              <div className="space-y-3">
                {results.map((result, index) => (
                  <div
                    key={`${result.chunkId}-${index}`}
                    className="rounded-lg border border-border bg-background p-4"
                  >
                    {/* Header: rank, doc, score */}
                    <div className="mb-2 flex flex-wrap items-center gap-2">
                      <Badge variant="secondary">#{index + 1}</Badge>
                      <span className="text-sm font-medium text-foreground">
                        {result.documentName}
                      </span>
                      <Badge variant="outline">相关度: {result.score.toFixed(4)}</Badge>
                      {result.chunkIndex != null && (
                        <span className="text-xs text-muted-foreground">
                          第{result.chunkIndex + 1}块
                        </span>
                      )}
                    </div>
                    {/* Text snippet */}
                    <p className="text-sm leading-relaxed text-muted-foreground">
                      {result.contentPreview}
                    </p>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}

        {/* Initial placeholder (before any search) */}
        {!searchMutation.isSuccess && !searchMutation.isError && !searchMutation.isPending && (
          <div className="mt-4 rounded-lg border border-dashed border-border p-10 text-center text-sm text-muted-foreground">
            检索结果将在此展示（来源文档、原文片段、相关性分数）
          </div>
        )}
      </div>
    </div>
  )
}
