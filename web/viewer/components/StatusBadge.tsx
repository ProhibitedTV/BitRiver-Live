interface StatusBadgeProps {
  label: string;
  tone?: "neutral" | "info" | "success" | "danger";
}

export function StatusBadge({ label, tone = "neutral" }: StatusBadgeProps) {
  return <span className={`badge status-badge status-badge--${tone}`}>{label}</span>;
}
