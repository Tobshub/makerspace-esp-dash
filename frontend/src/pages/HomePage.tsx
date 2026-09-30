import { Link } from 'react-router-dom'

export function HomePage() {
  return (
    <section className="page">
      <p className="eyebrow">ESP32 dashboard</p>
      <h1>Connect a device without changing the backend.</h1>
      <p className="lede">
        Teams register an ESP32, send arbitrary telemetry over MQTT, and shape
        the dashboard from metric definitions. This repository is the initial
        skeleton. Implementation follows the phases in <code>docs/phases</code>.
      </p>
      <p>
        <Link to="/login">Sign in</Link> is a placeholder until Phase 2.
      </p>
    </section>
  )
}
