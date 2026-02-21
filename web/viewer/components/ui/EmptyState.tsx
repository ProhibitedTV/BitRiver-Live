import { HTMLAttributes } from "react";

type EmptyStateProps = HTMLAttributes<HTMLDivElement>;

export function EmptyState({ className, ...props }: EmptyStateProps) {
  const mergedClassName = className ? `surface surface--empty stack ${className}` : "surface surface--empty stack";
  return <div className={mergedClassName} {...props} />;
}
