"use client";

import Link from "next/link";
import { FormEvent, useState } from "react";
import { useAuth } from "../hooks/useAuth";

export function Navbar() {
  const { user, login, signup, logout, error } = useAuth();
  const [mode, setMode] = useState<"login" | "signup" | null>(null);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [message, setMessage] = useState<string | undefined>();

  const reset = () => {
    setEmail("");
    setPassword("");
    setDisplayName("");
    setMessage(undefined);
  };

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (mode === "login") {
      const ok = await login(email, password);
      setMessage(ok ? undefined : "Unable to sign in. Check your credentials.");
      if (ok) {
        setMode(null);
        reset();
      }
    } else if (mode === "signup") {
      const ok = await signup(displayName, email, password);
      setMessage(ok ? undefined : "Unable to create account.");
      if (ok) {
        setMode(null);
        reset();
      }
    }
  };

  return (
    <header className="navbar">
      <div className="navbar-inner">
        <Link href="/" aria-label="BitRiver Live home" className="badge">
          <span role="img" aria-hidden>
            📡
          </span>
          BitRiver Live
        </Link>
        <nav className="nav-links">
          <Link href="/">Directory</Link>
          {user ? (
            <>
              <span className="muted">Signed in as {user.displayName}</span>
              <button className="secondary-button" onClick={() => logout()}>
                Sign out
              </button>
            </>
          ) : (
            <>
              <button
                className="secondary-button"
                onClick={() => {
                  reset();
                  setMode("login");
                }}
              >
                Sign in
              </button>
              <button
                className="primary-button"
                onClick={() => {
                  reset();
                  setMode("signup");
                }}
              >
                Create account
              </button>
            </>
          )}
        </nav>
      </div>
      {mode && (
        <div className="container" style={{ paddingTop: "0", paddingBottom: "2rem" }}>
          <form className="surface stack" onSubmit={handleSubmit}>
            <header className="stack">
              <h2>{mode === "login" ? "Welcome back" : "Join BitRiver Live"}</h2>
              <p className="muted">
                {mode === "login"
                  ? "Sign in to follow your favourite channels, sync subscriptions, and take part in chat."
                  : "Create a viewer account to follow creators, receive live notifications, and access subscriber features."}
              </p>
            </header>
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
            {(message || error) && <span className="muted">{message ?? error}</span>}
            <div className="nav-links" style={{ justifyContent: "flex-end" }}>
              <button
                type="button"
                className="secondary-button"
                onClick={() => {
                  setMode(null);
                  reset();
                }}
              >
                Cancel
              </button>
              <button type="submit" className="primary-button">
                {mode === "login" ? "Sign in" : "Create account"}
              </button>
            </div>
          </form>
        </div>
      )}
    </header>
  );
}
