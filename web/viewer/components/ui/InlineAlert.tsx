import { HTMLAttributes } from "react";

type InlineAlertTone = "error" | "info";

type InlineAlertProps = HTMLAttributes<HTMLParagraphElement> & {
  tone?: InlineAlertTone;
};

export function InlineAlert({ tone = "error", className, ...props }: InlineAlertProps) {
  const toneClass = tone === "error" ? "error" : "muted";
  const mergedClassName = className ? `${toneClass} ${className}` : toneClass;
  return <p className={mergedClassName} {...props} />;
}
