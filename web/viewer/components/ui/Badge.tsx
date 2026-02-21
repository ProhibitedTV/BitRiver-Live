import { HTMLAttributes } from "react";

type BadgeTone = "neutral" | "info" | "success" | "danger";

type BadgeProps = HTMLAttributes<HTMLSpanElement> & {
  tone?: BadgeTone;
};

function mergeClassName(base: string, className?: string): string {
  return className ? `${base} ${className}` : base;
}

export function Badge({ tone = "neutral", className, ...props }: BadgeProps) {
  return <span className={mergeClassName(`badge status-badge status-badge--${tone}`, className)} {...props} />;
}
