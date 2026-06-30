import type { ReactNode } from 'react'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

type ConfirmDialogProps = {
  cancelLabel?: ReactNode
  confirmLabel?: ReactNode
  description: ReactNode
  disabled?: boolean
  onConfirm: () => void
  onOpenChange: (open: boolean) => void
  open: boolean
  pending?: boolean
  pendingLabel?: ReactNode
  title: ReactNode
  variant?: 'default' | 'destructive'
}

export function ConfirmDialog({
  cancelLabel = '取消',
  confirmLabel = '确认',
  description,
  disabled,
  onConfirm,
  onOpenChange,
  open,
  pending,
  pendingLabel,
  title,
  variant = 'default',
}: ConfirmDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={pending}>
            {cancelLabel}
          </Button>
          <Button variant={variant} onClick={onConfirm} disabled={disabled || pending}>
            {pending ? (pendingLabel ?? confirmLabel) : confirmLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
