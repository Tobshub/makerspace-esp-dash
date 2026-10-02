import { useState } from 'react'

export function CopyButton({ value }: { value: string }) {
  const [label, setLabel] = useState('Copy')
  return (
    <button
      type="button"
      className="secondary"
      onClick={() => {
        void navigator.clipboard.writeText(value).then(
          () => {
            setLabel('Copied')
            window.setTimeout(() => setLabel('Copy'), 1500)
          },
          () => setLabel('Copy failed'),
        )
      }}
    >
      {label}
    </button>
  )
}
