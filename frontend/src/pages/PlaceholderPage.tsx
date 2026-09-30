export function PlaceholderPage({ title, phase }: { title: string; phase: string }) {
  return (
    <section className="page">
      <p className="eyebrow">{phase}</p>
      <h1>{title}</h1>
      <p className="lede">This screen is routed and waiting for its phase.</p>
    </section>
  )
}
