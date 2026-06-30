import { create } from 'zustand'

import { createSession, createUserSession, deleteCurrentSession, getCurrentUser } from '@/api/auth'
import { apiClient, ApiError } from '@/api/client'
import type { CreateSessionRequest, CreateUserRequest, UserSummary } from '@/lib/types'

export type AuthStatus = 'idle' | 'restoring' | 'authenticated' | 'anonymous' | 'error'

type AuthState = {
  accessToken: string | null
  error: string | null
  status: AuthStatus
  user: UserSummary | null
  userName: string | null
  clearSession: () => void
  login: (credentials: CreateSessionRequest) => Promise<void>
  logout: () => Promise<void>
  register: (credentials: CreateUserRequest) => Promise<void>
  restoreSession: () => Promise<void>
  setUserName: (userName: string | null) => void
}

let restorePromise: Promise<void> | null = null

function toErrorMessage(error: unknown): string {
  if (error instanceof ApiError) return error.message
  if (error instanceof Error) return error.message
  return '请求失败，请稍后重试'
}

export const useAuthStore = create<AuthState>((set) => ({
  accessToken: apiClient.getToken(),
  error: null,
  status: apiClient.getToken() ? 'idle' : 'anonymous',
  user: null,
  userName: null,
  clearSession: () => {
    apiClient.setToken(null)
    set({
      accessToken: null,
      error: null,
      status: 'anonymous',
      user: null,
      userName: null,
    })
  },
  login: async (credentials) => {
    set({ error: null, status: 'restoring' })
    try {
      const result = await createSession(credentials)
      apiClient.setToken(result.session.accessToken)
      set({
        accessToken: result.session.accessToken,
        error: null,
        status: 'authenticated',
        user: result.user,
        userName: result.user.username,
      })
    } catch (error) {
      apiClient.setToken(null)
      set({
        accessToken: null,
        error: toErrorMessage(error),
        status: 'anonymous',
        user: null,
        userName: null,
      })
      throw error
    }
  },
  logout: async () => {
    try {
      if (apiClient.getToken()) {
        await deleteCurrentSession()
      }
    } finally {
      useAuthStore.getState().clearSession()
    }
  },
  register: async (credentials) => {
    set({ error: null, status: 'restoring' })
    try {
      const result = await createUserSession(credentials)
      apiClient.setToken(result.session.accessToken)
      set({
        accessToken: result.session.accessToken,
        error: null,
        status: 'authenticated',
        user: result.user,
        userName: result.user.username,
      })
    } catch (error) {
      apiClient.setToken(null)
      set({
        accessToken: null,
        error: toErrorMessage(error),
        status: 'anonymous',
        user: null,
        userName: null,
      })
      throw error
    }
  },
  restoreSession: async () => {
    if (restorePromise) return restorePromise

    restorePromise = (async () => {
      const token = apiClient.getToken()
      if (!token) {
        set({
          accessToken: null,
          error: null,
          status: 'anonymous',
          user: null,
          userName: null,
        })
        return
      }

      // Dev bypass: skip API call, use mock user (only in development)
      if (import.meta.env.DEV && token === 'dev-token-bypass') {
        const mockUser: UserSummary = {
          id: 'dev',
          username: '开发者',
          roles: ['system:admin'],
          permissions: [
            'qa:use',
            'report:read',
            'report:write',
            'knowledge:read',
            'knowledge:write',
            'document:upload',
            'system:admin',
            'admin:model-profile:write',
            'admin:parser-config:write',
          ],
        }
        set({
          accessToken: token,
          error: null,
          status: 'authenticated',
          user: mockUser,
          userName: mockUser.username,
        })
        return
      }

      set({ accessToken: token, error: null, status: 'restoring' })

      try {
        const user = await getCurrentUser()
        set({
          accessToken: token,
          error: null,
          status: 'authenticated',
          user,
          userName: user.username,
        })
      } catch (error) {
        if (error instanceof ApiError && error.isUnauthorized()) {
          useAuthStore.getState().clearSession()
          return
        }

        set({
          accessToken: token,
          error: toErrorMessage(error),
          status: 'error',
          user: null,
          userName: null,
        })
      }
    })()

    try {
      await restorePromise
    } finally {
      restorePromise = null
    }
  },
  setUserName: (userName) => set({ userName }),
}))

apiClient.setAccessTokenProvider(() => useAuthStore.getState().accessToken ?? apiClient.getToken())
apiClient.setUnauthorizedHandler(() => {
  useAuthStore.setState({
    accessToken: null,
    error: null,
    status: 'anonymous',
    user: null,
    userName: null,
  })
})
