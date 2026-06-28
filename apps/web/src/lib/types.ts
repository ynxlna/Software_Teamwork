// ============ 通用 ============
export interface ApiResponse<T> {
  code: number
  message: string
  data: T
}

export interface PaginatedData<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}

// ============ 会话 ============
export interface Conversation {
  id: string
  title: string
  messages: Message[]
  created_at: string
  updated_at: string
}

export interface ConversationListItem {
  id: string
  title: string
  message_count: number
  last_message_preview: string
  created_at: string
  updated_at: string
}

// ============ 消息 ============
export interface Message {
  id: string
  role: 'user' | 'assistant' | 'system'
  content: string
  timestamp: string
  status?: 'streaming' | 'completed' | 'stopped' | 'failed'
  thinking?: ThinkingStep[]
  citations?: Citation[]
}

export interface ThinkingStep {
  type: 'intent' | 'retrieval' | 'generation' | 'verify'
  label: string
  status: 'pending' | 'running' | 'done'
  detail?: string
}

export interface Citation {
  id: string
  doc_id: string
  doc_name: string
  chunk_id: string
  text: string
  score: number
}

// ============ 聊天请求 ============
export interface ChatStreamRequest {
  conversation_id: string
  message: string
  knowledge_bases?: string[]
  params?: {
    top_k?: number
    similarity_threshold?: number
    use_rerank?: boolean
    rerank_threshold?: number
  }
}

// ============ 意图 ============
export type IntentType = 'knowledge_qa' | 'general_chat' | 'document_query' | 'system_command'

export interface IntentResult {
  intent: IntentType
  label: string
  confidence: number
  reasoning?: string
}

// ============ RAG 检索 ============
export interface RAGSearchRequest {
  query: string
  knowledge_bases?: string[]
  top_k?: number
  similarity_threshold?: number
  use_rerank?: boolean
  rerank_threshold?: number
  rerank_top_n?: number
}

export interface RAGSearchResult {
  rank: number
  chunk_id: string
  doc_id: string
  doc_name: string
  text: string
  vector_score: number
  rerank_score?: number
  page_number?: number
  chunk_index?: number
}

// ============ 文档 ============
export type DocumentStatus = 'uploading' | 'parsing' | 'chunking' | 'embedding' | 'ready' | 'failed'

export interface DocumentUploadResponse {
  doc_id: string
  filename: string
  file_size: number
  status: DocumentStatus
  knowledge_base: string
  uploaded_at: string
}

export interface DocumentStatusResponse {
  doc_id: string
  status: DocumentStatus
  progress: {
    step: string
    current: number
    total: number
    percent: number
  }
  error: string | null
}

// ============ 配置 ============
export interface KnowledgeBaseConfig {
  id: string
  name: string
  doc_count: number
  status: 'active' | 'inactive'
}

export interface RAGDefaults {
  knowledge_bases: string[]
  top_k: number
  similarity_threshold: number
  use_rerank: boolean
  rerank_threshold: number
  rerank_top_n: number
}

export interface LLMConfig {
  api_url: string
  api_key: string
  model_name: string
  timeout: number
  temperature: number
  max_tokens: number
}

// ============ 统计 ============
export interface StatsOverview {
  total_qa_count: number
  today_qa_count: number
  avg_latency_ms: number
  active_users_today: number
  knowledge_base_count: number
  document_count: number
}

export interface TrendPoint {
  date: string
  count: number
}

export interface TopQuery {
  query: string
  count: number
  avg_accuracy_score: number
  last_asked_at: string
}

// ============ SSE 事件类型 ============
export type SSEEventType =
  | 'intent_status'
  | 'thinking_step'
  | 'token'
  | 'citation'
  | 'done'
  | 'error'
  | 'heartbeat'

export interface SSEIntentStatusData {
  status: 'started' | 'done'
  label: string
  intent?: IntentType
  confidence?: number
  seq: number
}

export interface SSEThinkingStepData {
  step: ThinkingStep
  seq: number
}

export interface SSETokenData {
  text: string
  index: number
  seq: number
}

export interface SSECitationData {
  citation: Citation
  seq: number
}

export interface SSEDoneData {
  message_id: string
  total_tokens: number
  prompt_tokens: number
  completion_tokens: number
  latency_ms: number
  seq: number
}

export interface SSEErrorData {
  code: number
  message: string
  fatal: boolean
  seq: number
}

// ============ UI 类型 ============
export interface AdminMenuItem {
  key: string
  label: string
  icon?: string
  path?: string
  children?: AdminMenuItem[]
}
