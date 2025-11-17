"use client";

import { FormEvent, useEffect, useState } from "react";
import { createPortal } from "react-dom";

type AuthMode = "login" | "signup" | null;

interface AuthDialogProps {
  mode: AuthMode;
  onClose: () => void;
  onLogin: (email: string, password: string) => Promise<boolean>;
  onSignup: (displayName: string, email: string, password: string) => Promise<boolean>;
  authError?: string;
}

export function AuthDialog({ mode, onClose, onLogin, onSignup, authError }: AuthDialogProps) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [message, setMessage] = useState<string | undefined>();
  const [submitting, setSubmitting] = useState(false);
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setEmail("");
    setPassword("");
    setDisplayName("");
    setMessage(undefined);
    setSubmitting(false);
  }, [mode]);

  useEffect(() => {
    setMounted(true);
  }, []);

  if (!mode || !mounted) {
    return null;
  }

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setSubmitting(true);
    const trimmedEmail = email.trim();
    const trimmedDisplayName = displayName.trim();

    const ok =
      mode === "login"
        ? await onLogin(trimmedEmail, password)
        : await onSignup(trimmedDisplayName, trimmedEmail, password);

    if (ok) {
      onClose();
      setSubmitting(false);
      return;
    }

    setSubmitting(false);
    setMessage(mode === "login" ? "Unable to sign in. Check your credentials." : "Unable to create account.");
  };

  const dialog = (
    <div className="auth-dialog__backdrop" role="presentation" onClick={onClose}>
      <div
        className="auth-dialog"
        role="dialog"
        aria-modal="true"
        aria-label={mode === "login" ? "Sign in to BitRiver Live" : "Create a BitRiver Live account"}
        onClick={(event) => event.stopPropagation()}
      >
        <header className="auth-dialog__header">
          <div>
            <p className="muted auth-dialog__eyebrow">{mode === "login" ? "Welcome back" : "Join the community"}</p>
            <h2>{mode === "login" ? "Sign in" : "Create account"}</h2>
            <p className="muted auth-dialog__subtext">
              {mode === "login"
                ? "Access your subscriptions, chat across channels, and pick up where you left off."
                : "Follow your favourite creators, get live notifications, and unlock subscriber perks."}
            </p>
          </div>
          <button className="icon-button" type="button" onClick={onClose} aria-label="Close authentication dialog">
            ✕
          </button>
        </header>
        <form className="stack" onSubmit={handleSubmit}>
          {mode === "signup" && (
            <label className="stack">
              <span className="muted">Display name</span>
              <input
                type="text"
                required
                placeholder="Stream enthusiast"
                value={displayName}
                onChange={(event) => setDisplayName(event.target.value)}
              />
            </label>
          )}
          <label className="stack">
            <span className="muted">Email</span>
            <input
              type="email"
              required
              placeholder="you@example.com"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
            />
          </label>
          <label className="stack">
            <span className="muted">Password</span>
            <input
              type="password"
              required
              minLength={8}
              value={password}
              onChange={(event) => setPassword(event.target.value)}
            />
          </label>
          {(message || authError) && <span className="muted">{message ?? authError}</span>}
          <div className="auth-dialog__actions">
            <button type="button" className="ghost-button" onClick={onClose} disabled={submitting}>
              Cancel
            </button>
            <button type="submit" className="accent-button" disabled={submitting}>
              {mode === "login" ? "Sign in" : "Create account"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );

  return createPortal(dialog, document.body);
}
