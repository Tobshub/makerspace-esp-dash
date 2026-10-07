import { Link } from 'react-router-dom'

export function HomePage() {
  return (
    <section className="page">
      <p className="eyebrow">ESP32 dashboard</p>
      <h1>Connect a device without changing the backend.</h1>
      <p className="lede">
        Teams register an ESP32, send arbitrary telemetry over MQTT, and shape
        the dashboard from metric definitions. Sign in to create a team and project, then add a device.
      </p>
      <p className="row">
        <Link className="button" to="/login">
          Sign in
        </Link>
      </p>
    </section>
  )
}
