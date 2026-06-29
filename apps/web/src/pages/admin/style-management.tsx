import { Check, Moon, PaintBucket, Radius, Sun, Type } from 'lucide-react'

import type { ColorKey, ColorPreset } from '@/hooks'
import { FONT_SIZE_SCALES, PRIMARY_COLORS, useThemeSync } from '@/hooks'
import { cn } from '@/lib/utils'
import { useThemeStore } from '@/stores/theme-store'

/* -------------------------------------------------------------------------- */
/*  Helpers                                                                   */
/* -------------------------------------------------------------------------- */

const RADIUS_OPTIONS = [
  { value: 0, desc: '直角' },
  { value: 0.375, desc: '小' },
  { value: 0.625, desc: '默认' },
  { value: 1, desc: '大' },
] as const

const FONT_SIZE_OPTIONS = [
  { value: 'small', label: '小' },
  { value: 'medium', label: '中' },
  { value: 'large', label: '大' },
] as const

/* -------------------------------------------------------------------------- */
/*  Page                                                                      */
/* -------------------------------------------------------------------------- */

export function StyleManagement() {
  const mode = useThemeStore((s) => s.mode)
  const setMode = useThemeStore((s) => s.setMode)
  const primaryColor = useThemeStore((s) => s.primaryColor)
  const setPrimaryColor = useThemeStore((s) => s.setPrimaryColor)
  const radius = useThemeStore((s) => s.radius)
  const setRadius = useThemeStore((s) => s.setRadius)
  const fontSize = useThemeStore((s) => s.fontSize)
  const setFontSize = useThemeStore((s) => s.setFontSize)

  useThemeSync()

  /* ----- Render ----- */

  return (
    <div>
      <h3 className="mb-4 text-2xl font-semibold text-foreground">样式管理</h3>
      <p className="mb-6 text-sm text-muted-foreground">
        管理系统界面的主题样式、品牌标识和展示配置。
      </p>

      {/* ================================================================ */}
      {/*  Section 1 – 外观模式                                              */}
      {/* ================================================================ */}
      <section className="mb-8">
        <div className="mb-3 flex items-center gap-2">
          <Sun aria-hidden="true" className="size-4 text-muted-foreground" />
          <h4 className="text-base font-semibold text-foreground">外观模式</h4>
        </div>

        <div className="grid grid-cols-2 gap-3">
          {/* Light card */}
          <button
            type="button"
            onClick={() => setMode('light')}
            className={cn(
              'cursor-pointer rounded-xl border-2 p-4 text-left transition-all duration-200 hover:scale-[1.02]',
              mode === 'light'
                ? 'border-primary ring-2 ring-ring/30'
                : 'border-border hover:border-muted-foreground/30',
            )}
          >
            {/* Mini preview */}
            <div className="mb-3 overflow-hidden rounded-lg border border-border bg-white">
              <div className="h-2 bg-muted" />
              <div className="space-y-1.5 px-2 py-2">
                <div className="h-1.5 w-3/4 rounded bg-muted-foreground/20" />
                <div className="h-1.5 w-1/2 rounded bg-muted-foreground/15" />
                <div className="h-1.5 w-2/3 rounded bg-muted-foreground/15" />
              </div>
            </div>
            <div className="flex items-center gap-2">
              <Sun aria-hidden="true" className="size-4 text-foreground" />
              <span className="text-sm font-medium text-foreground">浅色模式</span>
              {mode === 'light' && (
                <Check aria-hidden="true" className="ml-auto size-4 text-primary" />
              )}
            </div>
          </button>

          {/* Dark card */}
          <button
            type="button"
            onClick={() => setMode('dark')}
            className={cn(
              'cursor-pointer rounded-xl border-2 p-4 text-left transition-all duration-200 hover:scale-[1.02]',
              mode === 'dark'
                ? 'border-primary ring-2 ring-ring/30'
                : 'border-border hover:border-muted-foreground/30',
            )}
          >
            {/* Mini preview */}
            <div className="mb-3 overflow-hidden rounded-lg border border-border bg-[oklch(0.205_0_0)]">
              <div className="h-2 bg-[oklch(0.269_0_0)]" />
              <div className="space-y-1.5 px-2 py-2">
                <div className="h-1.5 w-3/4 rounded bg-white/20" />
                <div className="h-1.5 w-1/2 rounded bg-white/10" />
                <div className="h-1.5 w-2/3 rounded bg-white/10" />
              </div>
            </div>
            <div className="flex items-center gap-2">
              <Moon aria-hidden="true" className="size-4 text-foreground" />
              <span className="text-sm font-medium text-foreground">深色模式</span>
              {mode === 'dark' && (
                <Check aria-hidden="true" className="ml-auto size-4 text-primary" />
              )}
            </div>
          </button>
        </div>
      </section>

      {/* ================================================================ */}
      {/*  Section 2 – 主题色                                                 */}
      {/* ================================================================ */}
      <section className="mb-8">
        <div className="mb-3 flex items-center gap-2">
          <PaintBucket aria-hidden="true" className="size-4 text-muted-foreground" />
          <h4 className="text-base font-semibold text-foreground">主题色</h4>
        </div>

        <div className="grid grid-cols-5 gap-3">
          {(Object.entries(PRIMARY_COLORS) as [ColorKey, ColorPreset][]).map(([key, preset]) => {
            const isSelected = primaryColor === key
            return (
              <button
                key={key}
                type="button"
                onClick={() => setPrimaryColor(key)}
                className="flex cursor-pointer flex-col items-center gap-1.5 rounded-lg py-2 transition-all duration-200 hover:scale-[1.02] hover:bg-muted/50"
              >
                <span
                  className={cn(
                    'relative flex size-9 items-center justify-center rounded-full border-2 transition-all',
                    isSelected ? 'border-primary ring-2 ring-ring/20' : 'border-transparent',
                  )}
                  style={{ backgroundColor: preset.light }}
                >
                  {isSelected && (
                    <Check
                      aria-hidden="true"
                      className="size-4"
                      style={{
                        color: key === 'yellow' ? 'oklch(0.205 0 0)' : 'oklch(0.985 0 0)',
                      }}
                    />
                  )}
                </span>
                <span
                  className={cn(
                    'text-xs transition-colors',
                    isSelected ? 'font-medium text-foreground' : 'text-muted-foreground',
                  )}
                >
                  {preset.label}
                </span>
              </button>
            )
          })}
        </div>
      </section>

      {/* ================================================================ */}
      {/*  Section 3 – 圆角                                                   */}
      {/* ================================================================ */}
      <section className="mb-8">
        <div className="mb-3 flex items-center gap-2">
          <Radius aria-hidden="true" className="size-4 text-muted-foreground" />
          <h4 className="text-base font-semibold text-foreground">圆角</h4>
        </div>

        <div className="flex flex-wrap items-start gap-4">
          {/* Segmented control */}
          <div className="flex rounded-lg border border-border bg-muted/50 p-0.5">
            {RADIUS_OPTIONS.map((opt) => (
              <button
                key={opt.value}
                type="button"
                onClick={() => setRadius(opt.value)}
                className={cn(
                  'cursor-pointer rounded-md px-3 py-1.5 text-sm font-medium transition-all',
                  radius === opt.value
                    ? 'bg-background text-foreground shadow-sm'
                    : 'text-muted-foreground hover:text-foreground',
                )}
              >
                {opt.desc}
              </button>
            ))}
          </div>

          {/* Live preview box */}
          <div className="flex min-w-0 flex-1 items-center gap-3">
            <div
              className="flex h-16 w-28 items-center justify-center border-2 border-dashed border-border bg-muted/30 text-xs text-muted-foreground transition-all"
              style={{ borderRadius: `${radius}rem` }}
            >
              预览
            </div>
            <span className="text-xs text-muted-foreground tabular-nums">{radius}rem</span>
          </div>
        </div>
      </section>

      {/* ================================================================ */}
      {/*  Section 4 – 字体大小                                                */}
      {/* ================================================================ */}
      <section className="mb-8">
        <div className="mb-3 flex items-center gap-2">
          <Type aria-hidden="true" className="size-4 text-muted-foreground" />
          <h4 className="text-base font-semibold text-foreground">字体大小</h4>
        </div>

        <div className="flex flex-wrap items-start gap-4">
          {/* Segmented control */}
          <div className="flex rounded-lg border border-border bg-muted/50 p-0.5">
            {FONT_SIZE_OPTIONS.map((opt) => (
              <button
                key={opt.value}
                type="button"
                onClick={() => setFontSize(opt.value)}
                className={cn(
                  'cursor-pointer rounded-md px-4 py-1.5 text-sm font-medium transition-all',
                  fontSize === opt.value
                    ? 'bg-background text-foreground shadow-sm'
                    : 'text-muted-foreground hover:text-foreground',
                )}
              >
                {opt.label}
              </button>
            ))}
          </div>

          {/* Preview text */}
          <div className="flex min-w-0 flex-1 items-center rounded-lg border border-border bg-muted/30 px-4 py-3">
            <p
              className="leading-relaxed text-foreground transition-all"
              style={{ fontSize: `calc(1rem * ${FONT_SIZE_SCALES[fontSize]})` }}
            >
              预览文字 Preview
            </p>
          </div>
        </div>
      </section>
    </div>
  )
}
