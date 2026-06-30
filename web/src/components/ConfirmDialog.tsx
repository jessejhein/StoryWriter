type Props = {
  open: boolean
  title: string
  message: string
  confirmLabel: string
  cancelLabel?: string
  onConfirm: () => void
  onCancel: () => void
}

export default function ConfirmDialog({ open, title, message, confirmLabel, cancelLabel = 'Keep editing', onConfirm, onCancel }: Props) {
  if (!open) {
    return null
  }
  return (
    <div className="dialog-backdrop" role="presentation">
      <div aria-describedby="confirm-dialog-message" aria-labelledby="confirm-dialog-title" aria-modal="true" className="dialog" role="dialog">
        <h2 id="confirm-dialog-title">{title}</h2>
        <p id="confirm-dialog-message">{message}</p>
        <div className="actions">
          <button type="button" className="secondary" onClick={onCancel}>{cancelLabel}</button>
          <button type="button" onClick={onConfirm}>{confirmLabel}</button>
        </div>
      </div>
    </div>
  )
}
