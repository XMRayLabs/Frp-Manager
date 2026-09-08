import type { ReactNode } from 'react'
export function PageHeading({
  title,
  description,
  actions,
}: {
  title: string
  description: string
  actions?: ReactNode
}) {
  return (
    <div className="mb-6 flex flex-wrap items-end justify-between gap-4">
      <div>
        <p className="mb-2 text-xs font-medium text-primary">工作空间 / {title}</p>
        <h2 className="text-2xl font-semibold tracking-tight">{title}</h2>
        <p className="mt-2 max-w-2xl text-sm leading-relaxed text-muted-foreground">{description}</p>
      </div>
      {actions && <div className="flex flex-wrap gap-2">{actions}</div>}
    </div>
  )
}
