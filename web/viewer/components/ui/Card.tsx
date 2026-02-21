import { HTMLAttributes } from "react";

type CardProps = HTMLAttributes<HTMLElement>;

type CardSectionProps = HTMLAttributes<HTMLDivElement>;

function mergeClassName(base: string, className?: string): string {
  return className ? `${base} ${className}` : base;
}

export function Card({ className, ...props }: CardProps) {
  return <section className={mergeClassName("surface stack", className)} {...props} />;
}

export function CardHeader({ className, ...props }: CardSectionProps) {
  return <header className={mergeClassName("stack", className)} {...props} />;
}

export function CardBody({ className, ...props }: CardSectionProps) {
  return <div className={mergeClassName("stack", className)} {...props} />;
}
