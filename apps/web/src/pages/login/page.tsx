import { useRouter } from '@tanstack/react-router'
import { Loader2 } from 'lucide-react'
import { type FormEvent, useEffect, useMemo, useState } from 'react'
import { z } from 'zod'

import { apiClient, ApiError } from '@/api/client'
import type { UserSummary } from '@/lib/types'
import { useAuthStore } from '@/stores/auth-store'

const authSchema = z.object({
  username: z.string().trim().min(1, '请输入用户名'),
  password: z.string().min(1, '请输入密码'),
})

type AuthMode = 'login' | 'register'

function toErrorMessage(error: unknown): string {
  if (error instanceof ApiError) return error.message
  if (error instanceof Error) return error.message
  return '认证失败，请稍后重试'
}

export function LoginPage() {
  const router = useRouter()
  const login = useAuthStore((state) => state.login)
  const register = useAuthStore((state) => state.register)
  const status = useAuthStore((state) => state.status)
  const storeError = useAuthStore((state) => state.error)
  const [mode, setMode] = useState<AuthMode>('login')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [formError, setFormError] = useState<string | null>(null)
  const isSubmitting = status === 'restoring'
  const error = formError ?? storeError

  const modeHint = useMemo(
    () =>
      mode === 'login'
        ? '使用 Gateway 会话接口创建当前登录会话。'
        : '注册接口会返回新用户和当前会话。',
    [mode],
  )

  useEffect(() => {
    if (status === 'authenticated') {
      void router.navigate({ to: '/' })
    }
  }, [router, status])

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setFormError(null)

    const parsed = authSchema.safeParse({ username, password })
    if (!parsed.success) {
      setFormError(parsed.error.issues[0]?.message ?? '表单校验失败')
      return
    }

    try {
      if (mode === 'login') {
        await login(parsed.data)
      } else {
        await register(parsed.data)
      }
      await router.navigate({ to: '/' })
    } catch (caught) {
      setFormError(toErrorMessage(caught))
    }
  }

  return (
    <>
      <style>{`
        @keyframes loginGradient {
          0% { background-position-x: 0; }
          100% { background-position-x: 280px; }
        }
        @keyframes drawText {
          0% { stroke-dashoffset: 1000; }
          100% { stroke-dashoffset: 0; }
        }
        @keyframes fillIn {
          0% { fill: transparent; }
          100% { fill: white; }
        }
        @keyframes floatParticle {
          0%, 100% { transform: translateY(0) scale(1); opacity: 0.4; }
          50% { transform: translateY(-20px) scale(2); opacity: 1; }
        }
        @keyframes fadeInUp {
          0% { opacity: 0; transform: translateY(8px); }
          100% { opacity: 1; transform: translateY(0); }
        }
        .subtitle-word {
          opacity: 0;
          animation: fadeInUp 0.6s ease-out forwards;
        }
        .underline-hover {
          position: relative;
        }
        .underline-hover::after {
          content: '';
          position: absolute;
          left: 0;
          bottom: -2px;
          width: 0;
          height: 1px;
          background: #60a5fa;
          transition: width 0.3s ease;
        }
        .underline-hover:hover::after {
          width: 100%;
        }
        .svg-title text {
          fill: transparent;
          stroke-dasharray: 1000;
          stroke-dashoffset: 1000;
          animation: drawText 4s ease-in-out forwards, fillIn 1.2s ease-in-out 1s forwards;
        }
      `}</style>
      <main className="flex min-h-screen items-center justify-center bg-[#1a1a2e] px-4 py-8">
        {/* Decorative background glows */}
        <div className="pointer-events-none fixed inset-0 overflow-hidden" aria-hidden>
          <div className="absolute -top-40 left-1/2 h-[500px] w-[500px] -translate-x-1/2 rounded-full bg-blue-500/10 blur-[100px]" />
          <div className="absolute top-1/3 -right-20 h-[300px] w-[300px] rounded-full bg-violet-500/10 blur-[80px]" />
          <div className="absolute -bottom-20 left-1/4 h-[300px] w-[300px] rounded-full bg-blue-400/5 blur-[80px]" />
          {/* Subtle grid */}
          <div
            className="absolute inset-0 opacity-[0.05]"
            style={{
              backgroundImage:
                'linear-gradient(rgba(255,255,255,0.1) 1px, transparent 1px), linear-gradient(90deg, rgba(255,255,255,0.1) 1px, transparent 1px)',
              backgroundSize: '60px 60px',
            }}
          />
          {/* Floating particles */}
          {Array.from({ length: 6 }).map((_, i) => (
            <div
              key={i}
              className="absolute h-1 w-1 rounded-full bg-blue-400/40"
              style={{
                left: `${10 + ((i * 17) % 90)}%`,
                top: `${15 + ((i * 23) % 80)}%`,
                animation: `floatParticle ${3 + i * 1.5}s ease-in-out ${i * 0.7}s infinite`,
              }}
            />
          ))}
        </div>

        {/* Login card */}
        <div className="relative z-10 w-full max-w-sm px-4 sm:px-0">
          {/* Title with SVG stroke-draw animation */}
          <div className="mb-8 text-center svg-title">
            <svg
              viewBox="0 0 500 90"
              className="mx-auto h-14 w-full max-w-[340px] sm:h-20 sm:max-w-[450px]"
              role="img"
              aria-label="电力行业知识助手"
            >
              <text
                x="50%"
                y="68"
                textAnchor="middle"
                className="stroke-blue-400 text-[40px] font-bold select-none tracking-[0.08em] sm:text-[56px] sm:tracking-[0.1em]"
                strokeWidth="1"
                style={{ paintOrder: 'stroke fill' }}
              >
                电力行业知识助手
              </text>
            </svg>
            <p className="mt-3 flex items-center justify-center gap-1 text-sm text-blue-400/70">
              {[
                { text: '智能问答', delay: '2.5s' },
                { text: '·', delay: '2.8s' },
                { text: '知识管理', delay: '3.0s' },
                { text: '·', delay: '3.2s' },
                { text: '报告生成', delay: '3.4s' },
              ].map((item, i) => (
                <span
                  key={i}
                  className={`subtitle-word ${item.text !== '·' ? 'underline-hover cursor-default' : ''}`}
                  style={{ animationDelay: item.delay }}
                >
                  {item.text}
                </span>
              ))}
            </p>
          </div>

          {/* Form */}
          <form className="space-y-6" onSubmit={handleSubmit}>
            {/* Username input */}
            <div className="group relative w-full pt-3">
              <input
                className="peer w-full border-0 border-b-2 border-white/30 bg-transparent py-2.5 text-base tracking-wider text-white outline-none transition-colors placeholder-shown:border-white/30 focus:border-blue-400"
                autoComplete="username"
                disabled={isSubmitting}
                id="username"
                placeholder=" "
                value={username}
                onChange={(e) => setUsername(e.target.value)}
              />
              <label
                htmlFor="username"
                className="pointer-events-none absolute left-0 top-[22px] uppercase tracking-wider text-white/50 transition-all duration-300 peer-focus:-translate-y-4 peer-focus:text-xs peer-focus:text-blue-400 peer-[:not(:placeholder-shown)]:-translate-y-4 peer-[:not(:placeholder-shown)]:text-xs peer-[:not(:placeholder-shown)]:text-blue-400"
              >
                用户名
              </label>
              {/* Animated gradient underline */}
              <span className="absolute bottom-0 left-0 h-[2px] w-full overflow-hidden">
                <span
                  className="absolute left-[-100%] h-full w-full transition-[left] duration-500 group-focus-within:left-0"
                  style={{
                    background: 'linear-gradient(90deg, #fff, #3b82f6)',
                    backgroundSize: '280px 100%',
                    animation: 'loginGradient 2s linear infinite',
                  }}
                />
              </span>
            </div>

            {/* Password input */}
            <div className="group relative w-full pt-3">
              <input
                className="peer w-full border-0 border-b-2 border-white/30 bg-transparent py-2.5 text-base tracking-wider text-white outline-none transition-colors focus:border-blue-400"
                autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
                disabled={isSubmitting}
                id="password"
                placeholder=" "
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
              <label
                htmlFor="password"
                className="pointer-events-none absolute left-0 top-[22px] uppercase tracking-wider text-white/50 transition-all duration-300 peer-focus:-translate-y-4 peer-focus:text-xs peer-focus:text-blue-400 peer-[:not(:placeholder-shown)]:-translate-y-4 peer-[:not(:placeholder-shown)]:text-xs peer-[:not(:placeholder-shown)]:text-blue-400"
              >
                密码
              </label>
              {/* Animated gradient underline */}
              <span className="absolute bottom-0 left-0 h-[2px] w-full overflow-hidden">
                <span
                  className="absolute left-[-100%] h-full w-full transition-[left] duration-500 group-focus-within:left-0"
                  style={{
                    background: 'linear-gradient(90deg, #fff, #3b82f6)',
                    backgroundSize: '280px 100%',
                    animation: 'loginGradient 2s linear infinite',
                  }}
                />
              </span>
            </div>

            {/* Submit button */}
            <button
              type="submit"
              disabled={isSubmitting}
              className="group/btn h-12 w-full rounded bg-gradient-to-r from-blue-600 to-blue-500 font-medium tracking-widest text-white shadow-[0_0_20px_rgba(59,130,246,0.3)] transition-all duration-300 hover:from-blue-500 hover:to-blue-400 hover:shadow-[0_0_35px_rgba(59,130,246,0.5)] active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-50"
            >
              {isSubmitting ? <Loader2 className="mr-2 inline size-4 animate-spin" /> : null}
              {isSubmitting ? '请稍候' : mode === 'login' ? '登 录' : '注 册'}
            </button>

            {/* Mode toggle */}
            <button
              type="button"
              className="w-full py-2 text-center text-sm text-white/50 transition-colors hover:text-blue-400"
              disabled={isSubmitting}
              onClick={() => {
                setFormError(null)
                setMode((current) => (current === 'login' ? 'register' : 'login'))
              }}
            >
              {mode === 'login' ? '创建账号' : '返回登录'}
            </button>
          </form>

          {/* Error display */}
          {error && (
            <div className="mt-4 rounded-md border border-red-500/40 bg-red-500/10 px-3 py-2 text-sm text-red-400">
              {error}
            </div>
          )}

          {/* Dev bypass — only available in development */}
          {import.meta.env.DEV && (
            <button
              type="button"
              className="mt-4 w-full rounded-lg border border-dashed border-white/20 py-2 text-xs text-white/30 transition-colors hover:border-white/40 hover:text-white/50"
              onClick={() => {
                apiClient.setToken('dev-token-bypass')
                useAuthStore.setState({
                  accessToken: 'dev-token-bypass',
                  error: null,
                  status: 'authenticated',
                  user: {
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
                  } as UserSummary,
                  userName: '开发者',
                })
                void router.navigate({ to: '/' })
              }}
            >
              跳过登录（开发模式）
            </button>
          )}

          {/* Mode hint */}
          <p className="mt-4 text-center text-xs text-white/20">{modeHint}</p>
        </div>
      </main>
    </>
  )
}
