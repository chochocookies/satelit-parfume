import type { LucideIcon } from "lucide-react";

export function StatCard({
  label,
  value,
  hint,
  icon: Icon,
}: {
  label: string;
  value: string;
  hint?: string;
  icon?: LucideIcon;
}) {
  return (
    <div className="animate-fade-in-up rounded-2xl border border-line bg-surface p-5 transition hover:border-accent/50">
      <div className="flex items-center justify-between">
        <p className="text-sm text-ink-muted">{label}</p>
        {Icon && <Icon className="h-4 w-4 text-accent" aria-hidden="true" />}
      </div>
      <p className="mt-2 font-display text-2xl text-ink">{value}</p>
      {hint && <p className="mt-1 text-xs text-ink-muted">{hint}</p>}
    </div>
  );
}
