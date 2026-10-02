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
  return (
    <div style={{ width: '100%', height }}>
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={data}>
          {compact ? null : <XAxis dataKey="t" minTickGap={28} />}
          {compact ? null : <YAxis width={44} />}
          {compact ? null : <Tooltip />}
          <Line type="monotone" dataKey="v" stroke="#c45c26" dot={false} strokeWidth={2} isAnimationActive={false} />
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
}
