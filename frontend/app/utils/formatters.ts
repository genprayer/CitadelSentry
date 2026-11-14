export function formatPlayerRatio(current: number, max: number): string {
  return `${current} / ${max}`
}

export function formatPercentage(current: number, total: number): number {
  if (!total || total <= 0) return 0
  return Math.min(100, Math.max(0, Math.round((current / total) * 100)))
}

export function formatPing(pingMs?: number): string {
  if (pingMs === undefined || pingMs === null) return 'N/A'
  return `${pingMs}ms`
}
