import { useQueries } from '@tanstack/react-query'
import { deviceTelemetry, type MetricDefinition, type TelemetryValue } from '../api/client'
import { formatDefined, formatWhen, rangeMs } from '../format'
import { liveInterval } from '../live'
import { useNow } from '../useNow'
import { TelemetryChart } from './TelemetryChart'

export type WidgetDevice = {
  deviceId: string
  deviceName: string
  metrics: TelemetryValue[]
}

export function MetricWidgets({
  definitions,
  devices,
}: {
  definitions: MetricDefinition[]
  devices: WidgetDevice[]
}) {
  const now = useNow()
  const widgets = buildWidgets(definitions, devices)
  const from = new Date(now - rangeMs('24h')).toISOString()
  const series = useQueries({
    queries: widgets.map((widget) => ({
      queryKey: ['telemetry-series', widget.deviceId, widget.definition.key, 'widget-24h'],
      queryFn: () =>
        deviceTelemetry(widget.deviceId, {
          metric: widget.definition.key,
          from,
          limit: 200,
        }),
      enabled: widget.definition.displayType === 'line' && widget.deviceId !== '' && widget.value?.numericValue != null,
      refetchInterval: liveInterval(),
    })),
  })

  if (widgets.length === 0) return null

  return (
    <div className="card-grid">
      {widgets.map((widget, index) => (
        <WidgetCard
          key={`${widget.definition.id}-${widget.deviceId || 'waiting'}`}
          widget={widget}
          points={series[index]?.data?.points ?? []}
          now={now}
        />
      ))}
    </div>
  )
}

function WidgetCard({
  widget,
  points,
  now,
}: {
  widget: WidgetItem
  points: { numericValue: number | null; booleanValue: boolean | null; stringValue: string | null; recordedAt: string; receivedAt: string }[]
  now: number
}) {
  const { definition, deviceName, value } = widget
  const title = deviceName ? `${definition.name} · ${deviceName}` : definition.name
  const when = value ? formatWhen(value.recordedAt, now) : 'Waiting for telemetry'
  const text = formatDefined(value, definition)

  if (definition.displayType === 'line') {
    return (
      <div className="card widget">
        <p className="muted">{title}</p>
        <p className="stat-value">{text}</p>
        <p className="muted">{when}</p>
        {value?.numericValue != null ? <TelemetryChart points={points} height={120} /> : null}
      </div>
    )
  }

  if (definition.displayType === 'gauge') {
    const numeric = value?.numericValue
    const min = definition.minValue
    const max = definition.maxValue
    const span = min != null && max != null ? max - min : 0
    const pct = numeric == null || span <= 0 || min == null ? 0 : Math.min(100, Math.max(0, ((numeric - min) / span) * 100))
    return (
      <div className="card widget">
        <p className="muted">{title}</p>
        <p className="stat-value">{text}</p>
        <div className="gauge-track" role="meter" aria-valuemin={min ?? undefined} aria-valuemax={max ?? undefined} aria-valuenow={numeric ?? undefined} aria-label={definition.name}>
          <div className="gauge-fill" style={{ width: `${pct}%` }} />
        </div>
        <p className="muted">
          {min ?? '—'} – {max ?? '—'} · {when}
        </p>
      </div>
    )
  }

  if (definition.displayType === 'boolean' || definition.displayType === 'status') {
    const on = value?.booleanValue === true
    const off = value?.booleanValue === false
    return (
      <div className="card widget">
        <p className="muted">{title}</p>
        {on || off ? <p className={on ? 'pill on' : 'pill off'}>{on ? 'ON' : 'OFF'}</p> : <p className="stat-value">—</p>}
        <p className="muted">{when}</p>
        {definition.description ? <p className="muted">{definition.description}</p> : null}
      </div>
    )
  }

  if (definition.displayType === 'text') {
    return (
      <div className="card widget">
        <p className="muted">{title}</p>
        <p className="widget-text">{value?.stringValue ?? '—'}</p>
        <p className="muted">{when}</p>
      </div>
    )
  }

  return (
    <div className="card widget">
      <p className="muted">{title}</p>
      <p className="stat-value">{text}</p>
      <p className="muted">{when}</p>
      {definition.description ? <p className="muted">{definition.description}</p> : null}
    </div>
  )
}

type WidgetItem = {
  definition: MetricDefinition
  deviceId: string
  deviceName: string
  value: TelemetryValue | null
}

function buildWidgets(definitions: MetricDefinition[], devices: WidgetDevice[]): WidgetItem[] {
  const widgets: WidgetItem[] = []
  for (const definition of definitions) {
    let matched = false
    for (const device of devices) {
      const value = device.metrics.find((item) => item.metric === definition.key)
      if (!value) continue
      matched = true
      widgets.push({ definition, deviceId: device.deviceId, deviceName: device.deviceName, value })
    }
    if (!matched) {
      widgets.push({ definition, deviceId: '', deviceName: '', value: null })
    }
  }
  return widgets
}
