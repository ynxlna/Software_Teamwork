import { Send } from 'lucide-react'
import { type KeyboardEvent, useCallback, useEffect, useRef } from 'react'

import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'

type ChatInputProps = {
  onSend: (text: string) => void
  disabled: boolean
  value: string
  onChange: (value: string) => void
}

export default function ChatInput({ onSend, disabled, value, onChange }: ChatInputProps) {
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  // Auto-resize on text change
  useEffect(() => {
    const el = textareaRef.current
    if (!el) return
    el.style.height = 'auto'
    el.style.height = `${Math.min(el.scrollHeight, 160)}px`
  }, [value])

  const handleSend = useCallback(() => {
    const trimmed = value.trim()
    if (!trimmed || disabled) return
    onSend(trimmed)
    onChange('')
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto'
    }
  }, [value, disabled, onSend, onChange])

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  const canSend = value.trim().length > 0 && !disabled

  return (
    <div className="shrink-0 border-t border-border bg-background px-6 py-4">
      <div className="flex items-end gap-2 rounded-lg border border-primary/30 bg-card px-3 py-2 transition-colors focus-within:border-primary focus-within:ring-3 focus-within:ring-primary/30">
        <Textarea
          ref={textareaRef}
          className="min-h-0 flex-1 resize-none border-0 bg-transparent p-0 py-1 text-base placeholder:text-muted-foreground focus-visible:ring-0 disabled:cursor-not-allowed disabled:opacity-60 md:text-sm"
          placeholder="输入您的问题… (Enter 发送，Shift+Enter 换行)"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onKeyDown={handleKeyDown}
          disabled={disabled}
          rows={1}
        />
        <Button
          size="icon"
          onClick={handleSend}
          disabled={!canSend}
          className="shrink-0 rounded-full bg-primary text-primary-foreground transition-all duration-200 hover:bg-primary/90 hover:scale-110 hover:shadow-md active:scale-90"
          aria-label="发送消息"
        >
          <Send className="size-4" aria-hidden="true" />
        </Button>
      </div>
    </div>
  )
}
