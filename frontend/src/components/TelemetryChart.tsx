import { Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import type { TelemetryPoint } from '../api/client'

export function TelemetryChart({
  points,
  height = 240,
  compact = false,
}: {
  points: TelemetryPoint[]
  height?: number
  compact?: boolean
}) {
  const data = points
    .filter((point) => point.numericValue != null)
    .map((point) => ({
      t: new Date(point.recordedAt).toLocaleString([], compact ? { hour: '2-digit', minute: '2-digit' } : undefined),
      v: point.numericValue,
    }))
  if (data.length === 0) return <p className="muted">No points in this window.</p>
  const tick = { fill: 'var(--dim)', fontSize: 11, fontFamily: 'JetBrains Mono, ui-monospace, monospace' }
  return (
    <div style={{ width: '100%', height }}>
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={data}>
          {compact ? null : <XAxis dataKey="t" minTickGap={28} stroke="var(--line-2)" tick={tick} />}
          {compact ? null : <YAxis width={48} stroke="var(--line-2)" tick={tick} />}
          {compact ? null : (
            <Tooltip
              contentStyle={{
                background: 'var(--panel)',
                border: '1px solid var(--line)',
                borderRadius: 2,
                color: 'var(--text)',
                fontFamily: 'JetBrains Mono, ui-monospace, monospace',
                fontSize: 12,
              }}
            />
          )}
          <Line type="monotone" dataKey="v" stroke="var(--accent)" dot={false} strokeWidth={2} isAnimationActive={false} />
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
}
