import { appendHash, resolveSignupUrl } from "../../lib/auth-links";

export type FollowingStatus = "loading" | "unauthenticated" | "error" | "empty" | "ready";

export type FollowingCallToAction = {
  label: string;
  href: string;
};

export const FOLLOWING_SIGN_IN_CTA: FollowingCallToAction = {
  label: "Sign in",
  href: appendHash(resolveSignupUrl() ?? "/signup", "login-form"),
};

export const FOLLOWING_COPY = {
  loading: "Checking who is live...",
  unauthenticated: "Sign in to see channels you follow.",
  empty: "You're not following any channels yet.",
  error: "We couldn't load your followed channels.",
  retry: "Try again",
  summaryFollowed: (count: number) => `${count} followed`,
  summaryLiveNow: (count: number) => `${count} live now`,
};

export function FollowingLoadingBlock({ className }: { className?: string }) {
  return <p className={className}>{FOLLOWING_COPY.loading}</p>;
}

export function FollowingUnauthenticatedPrompt({ className }: { className?: string }) {
  return (
    <div className={className}>
      <p>{FOLLOWING_COPY.unauthenticated}</p>
      <a href={FOLLOWING_SIGN_IN_CTA.href} className="primary-button">
        {FOLLOWING_SIGN_IN_CTA.label}
      </a>
    </div>
  );
}

export function FollowingEmptyPrompt({ className }: { className?: string }) {
  return <p className={className}>{FOLLOWING_COPY.empty}</p>;
}

export function FollowingErrorBlock({ className, onRetry }: { className?: string; onRetry?: () => void }) {
  return (
    <div className={className} role="status">
      <p>{FOLLOWING_COPY.error}</p>
      {onRetry ? (
        <button type="button" onClick={onRetry} className="following-sidebar__retry">
          {FOLLOWING_COPY.retry}
        </button>
      ) : null}
    </div>
  );
}
