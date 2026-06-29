/**
 * Admin API — Gateway OpenAPI admin-runtime-config, qa-settings,
 * qa-retrieval-tests, qa-metrics, knowledge, auth paths.
 *
 * All functions use gatewayRequest / gatewayPageRequest from ./client.
 * Types imported from @/lib/types (camelCase, per OpenAPI).
 */

import type {
  CreateKnowledgeBaseRequest,
  CreateQAConfigVersionRequest,
  CreateQALLMConfigVersionRequest,
  CreateUserRequest,
  KnowledgeBaseSummary,
  QAConfigVersion,
  QAIntentDistributionItem,
  QALLMConfigVersion,
  QALLMConnectionTest,
  QALLMConnectionTestRequest,
  QAMetricsOverview,
  QAMetricsTrend,
  QARetrievalTestRun,
  QARetrievalTestRunRequest,
  QATopQuery,
  SessionSummary,
  UserSummary,
} from '@/lib/types'

import { buildQuery, gatewayPageRequest, gatewayRequest } from './client'

// =========================================================================
// LLM Configuration
// =========================================================================

/** GET /llm-config-versions/current */
export function getCurrentLLMConfig(): Promise<QALLMConfigVersion> {
  return gatewayRequest<QALLMConfigVersion>('/llm-config-versions/current')
}

/** POST /llm-config-versions */
export async function createLLMConfigVersion(
  config: CreateQALLMConfigVersionRequest,
): Promise<QALLMConfigVersion> {
  return gatewayRequest<QALLMConfigVersion>('/llm-config-versions', {
    method: 'POST',
    body: JSON.stringify(config),
  })
}

/** POST /llm-connection-tests */
export async function testLLMConnection(
  params: QALLMConnectionTestRequest,
): Promise<QALLMConnectionTest> {
  return gatewayRequest<QALLMConnectionTest>('/llm-connection-tests', {
    method: 'POST',
    body: {
      provider: 'ai-gateway',
      profileId,
      modelName: config.model_name ?? current.model_name,
      timeoutSeconds: config.timeout ?? current.timeout,
      temperature: config.temperature ?? current.temperature,
      maxTokens: config.max_tokens ?? current.max_tokens,
      activate: true,
    } satisfies components['schemas']['CreateQALLMConfigVersionRequest'],
  })

  return toLLMConfig(next, await getDefaultChatProfile().catch(() => undefined))
}

// =========================================================================
// QA Configuration
// =========================================================================

/** GET /qa-config-versions/current */
export function getCurrentQAConfig(): Promise<QAConfigVersion> {
  return gatewayRequest<QAConfigVersion>('/qa-config-versions/current')
}

/** POST /qa-config-versions */
export async function createQAConfigVersion(
  config: CreateQAConfigVersionRequest,
): Promise<QAConfigVersion> {
  return gatewayRequest<QAConfigVersion>('/qa-config-versions', {
    method: 'POST',
    body: JSON.stringify(config),
  })
}

// =========================================================================
// Retrieval Test
// =========================================================================

/** POST /retrieval-test-runs */
export async function runRetrievalTest(
  params: QARetrievalTestRunRequest,
): Promise<QARetrievalTestRun> {
  return gatewayRequest<QARetrievalTestRun>('/retrieval-test-runs', {
    method: 'POST',
    body: {
      defaultKnowledgeBaseIds: defaults.knowledge_bases,
      retrieval: {
        topK: defaults.top_k,
        scoreThreshold: defaults.similarity_threshold,
        enableRerank: defaults.use_rerank,
        useRerank: defaults.use_rerank,
        rerankTopN: defaults.rerank_top_n,
      },
      activate: true,
    } satisfies components['schemas']['CreateQAConfigVersionRequest'],
  })
  return getKnowledgeConfig()
}

// =========================================================================
// QA Metrics
// =========================================================================

/** GET /qa-metrics/overview?days=N */
export function getQAMetricsOverview(days?: number): Promise<QAMetricsOverview> {
  return gatewayRequest<QAMetricsOverview>(
    `/qa-metrics/overview${buildQuery({ days })}`,
  )
}

/** GET /qa-metrics/trend?days=N */
export function getQAMetricsTrend(days?: number): Promise<QAMetricsTrend> {
  return gatewayRequest<QAMetricsTrend>(
    `/qa-metrics/trend${buildQuery({ days: days ?? 30 })}`,
  )
}

/** GET /qa-metrics/top-queries?limit=N&days=N */
export async function getQATopQueries(limit?: number, days?: number): Promise<QATopQuery[]> {
  return gatewayRequest<QATopQuery[]>(
    `/qa-metrics/top-queries${buildQuery({ limit, days })}`,
  )
}

/** GET /qa-metrics/intent-distribution?days=N */
export async function getQAIntentDistribution(days?: number): Promise<QAIntentDistributionItem[]> {
  return gatewayRequest<QAIntentDistributionItem[]>(
    `/qa-metrics/intent-distribution${buildQuery({ days })}`,
  )
}

// =========================================================================
// Knowledge Bases
// =========================================================================

/** GET /knowledge-bases?page=&pageSize= */
export interface ListKnowledgeBasesParams {
  page?: number
  pageSize?: number
}

export async function listKnowledgeBases(
  params: ListKnowledgeBasesParams = {},
): Promise<{ items: KnowledgeBaseSummary[]; page: { page: number; pageSize: number; total: number } }> {
  return gatewayPageRequest<KnowledgeBaseSummary>(
    `/knowledge-bases${buildQuery({ page: params.page, pageSize: params.pageSize })}`,
  )
}

/** POST /knowledge-bases */
export async function createKnowledgeBase(
  params: CreateKnowledgeBaseRequest,
): Promise<KnowledgeBaseSummary> {
  return gatewayRequest<KnowledgeBaseSummary>('/knowledge-bases', {
    method: 'POST',
    body: {
      name: params.name,
      description: params.description ?? '',
    },
  })
  return toKnowledgeBaseConfig(kb)
}

/** DELETE /knowledge-bases/{knowledgeBaseId} */
export async function deleteKnowledgeBase(id: string): Promise<void> {
  await gatewayRequest<void>(`/knowledge-bases/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

// =========================================================================
// User Management — gateway auth paths
//
// The gateway exposes only these auth resources through /api/v1:
//   POST   /users              — Create user (registration)
//   GET    /users/me           — Get current user
//   POST   /sessions           — Create session (login)
//   DELETE /sessions/current   — Delete current session (logout)
//
// There are no list-all-users, update-user, delete-user, or role CRUD
// endpoints in the current gateway contract.
// =========================================================================

/** POST /users — Create a new user (registration). Returns user + session (envelope unwrapped). */
export async function createUser(body: CreateUserRequest): Promise<{
  user: UserSummary
  session: SessionSummary
}> {
  return gatewayRequest<{ user: UserSummary; session: SessionSummary }>('/users', {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

/** GET /users/me — Get current authenticated user. */
export function getCurrentUser(): Promise<UserSummary> {
  return gatewayRequest<UserSummary>('/users/me')
}
