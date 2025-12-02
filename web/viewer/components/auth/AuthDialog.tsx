"use client";

import { useEffect } from "react";
import { useAuth } from "../../hooks/useAuth";

type AuthDialogProps = {
  open: boolean;
  onClose: () => void;
  redirectTo?: string;
  title?: string;
  description?: string;
};

export function AuthDialog({ open, onClose, redirectTo, title, description }: AuthDialogProps) {
  const { user, loading, error, signIn, signOut } = useAuth();

  useEffect(() => {
    if (!open) {
      return undefined;
    }

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onClose();
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [open, onClose]);

  if (!open) {
    return null;
  }

  const handleSignIn = () => {
    void signIn(redirectTo);
  };

  const handleSignOut = () => {
    void signOut();
  };

  return (
    <div className="chat-panel__dialog-backdrop" role="presentation" onClick={onClose}>
      <section
        className="chat-panel__dialog surface"
        role="dialog"
        aria-modal="true"
        aria-labelledby="auth-dialog-title"
        onClick={(event) => event.stopPropagation()}
      >
        <header className="chat-panel__dialog-header">
          <h4 id="auth-dialog-title">{title ?? (user ? "Signed in" : "Sign in to continue")}</h4>
          <button type="button" className="icon-button" onClick={onClose} aria-label="Close sign-in dialog">
            ✕
          </button>
        </header>
        {description ? <p className="muted">{description}</p> : null}
        {error ? (
          <p className="error" role="alert">
            {error}
          </p>
        ) : null}
        <div className="chat-panel__dialog-actions">
          {user ? (
            <button type="button" className="ghost-button" onClick={handleSignOut} disabled={loading}>
              Sign out
            </button>
          ) : (
            <button type="button" className="accent-button" onClick={handleSignIn} disabled={loading}>
              {loading ? "Connecting…" : "Sign in"}
            </button>
          )}
          <button type="button" className="ghost-button" onClick={onClose}>
            Close
          </button>
        </div>
      </section>
    </div>
  );
}
