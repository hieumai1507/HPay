interface PlaceholderProps {
  title: string;
  phase: string;
  description: string;
}

/** Shell page used until the real screen ships in its phase (A2–A4). */
export function Placeholder({ title, phase, description }: PlaceholderProps) {
  return (
    <div
      style={{
        background: 'var(--card-bg)',
        border: '1px dashed var(--n200)',
        borderRadius: 'var(--radius)',
        padding: '48px 32px',
        textAlign: 'center',
      }}
    >
      <h2 style={{ margin: 0, fontSize: 18, fontWeight: 700 }}>{title}</h2>
      <p style={{ color: 'var(--n500)', margin: '8px auto 0', maxWidth: 480 }}>{description}</p>
      <span
        style={{
          display: 'inline-block',
          marginTop: 16,
          padding: '3px 12px',
          borderRadius: 99,
          fontSize: 12,
          fontWeight: 700,
          background: 'var(--brand-50)',
          color: 'var(--brand)',
        }}
      >
        Sẽ hoàn thiện ở phase {phase}
      </span>
    </div>
  );
}
