"use client";

import { FormEvent, useEffect, useRef, useState } from "react";
import { useAuth } from "../../hooks/useAuth";

function formatDestinationPath(route: string) {
  return route.length <= 56 ? route : `${route.slice(0, 53)}...`;
}

export function AuthDialog() {
  const {
    allowSelfSignup,
    authDialogOpen,
    authFeedback,
    authMode,
    authRedirectTo,
    closeAuthDialog,
    loading,
    mfaEnrollment,
    mfaRequired,
    setAuthMode,
    signOut,
    submitMFAVerification,
    submitSignIn,
    submitSignUp,
    user,
  } = useAuth();
  const [signInEmail, setSignInEmail] = useState("");
  const [signInPassword, setSignInPassword] = useState("");
  const [signUpDisplayName, setSignUpDisplayName] = useState("");
  const [signUpEmail, setSignUpEmail] = useState("");
  const [signUpPassword, setSignUpPassword] = useState("");
  const [mfaCode, setMFACode] = useState("");
  const initialFocusRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    if (!authDialogOpen) {
      return undefined;
    }

    initialFocusRef.current?.focus();
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        closeAuthDialog();
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [authDialogOpen, closeAuthDialog]);

  useEffect(() => {
    if (!authDialogOpen) {
      setSignInPassword("");
      setSignUpPassword("");
      setMFACode("");
    }
  }, [authDialogOpen]);

  useEffect(() => {
    setMFACode("");
  }, [mfaRequired]);

  const destinationPath = formatDestinationPath(authRedirectTo);
  const title = user ? "Signed in to BitRiver Live" : mfaRequired ? "Verify your account" : "Sign in to BitRiver Live";

  if (!authDialogOpen) {
    return null;
  }

  const handleSignInSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    await submitSignIn({ email: signInEmail, password: signInPassword });
  };

  const handleSignUpSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    await submitSignUp({ displayName: signUpDisplayName, email: signUpEmail, password: signUpPassword });
  };

  const handleMFASubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    await submitMFAVerification(mfaCode);
  };

  return (
    <div className="auth-overlay" role="presentation">
      <button type="button" className="auth-overlay__backdrop" aria-label="Close sign-in dialog" onClick={closeAuthDialog} />
      <section className="auth-overlay__dialog surface" role="dialog" aria-modal="true" aria-labelledby="viewer-auth-title">
        <header className="auth-overlay__header">
          <div className="stack stack--2xs">
            <span className="navbar-context__eyebrow">Viewer access</span>
            <h2 id="viewer-auth-title">{title}</h2>
          </div>
          <button type="button" className="ghost-button auth-overlay__close" onClick={closeAuthDialog}>
            Close
          </button>
        </header>

        <div className="auth-overlay__route surface">
          <div className="stack stack--2xs">
            <span className="navbar-context__eyebrow">Continue where you left off</span>
            <strong>{destinationPath}</strong>
          </div>
        </div>

        {user ? (
          <div className="auth-overlay__signed-in surface">
            <div className="stack stack--2xs">
              <span className="navbar-context__eyebrow">Signed in as</span>
              <strong>{user.displayName}</strong>
              <p className="muted">You can close this dialog or sign out from here if you switched accounts by mistake.</p>
            </div>
            <div className="auth-overlay__actions">
              <button type="button" className="ghost-button" onClick={() => void signOut()} disabled={loading}>
                Sign out
              </button>
              <button type="button" className="accent-button" onClick={closeAuthDialog}>
                Continue
              </button>
            </div>
          </div>
        ) : (
          <>
            {!mfaRequired && (
              <div className="auth-overlay__tabs" role="tablist" aria-label="Auth mode">
                <button
                  type="button"
                  role="tab"
                  className={`auth-overlay__tab${authMode === "signin" ? " auth-overlay__tab--active" : ""}`}
                  aria-selected={authMode === "signin"}
                  onClick={() => setAuthMode("signin")}
                >
                  Sign in
                </button>
                {allowSelfSignup && (
                  <button
                    type="button"
                    role="tab"
                    className={`auth-overlay__tab${authMode === "signup" ? " auth-overlay__tab--active" : ""}`}
                    aria-selected={authMode === "signup"}
                    onClick={() => setAuthMode("signup")}
                  >
                    Sign up
                  </button>
                )}
              </div>
            )}

            {authFeedback ? (
              <p className={`auth-overlay__feedback${authFeedback.variant === "error" ? " auth-overlay__feedback--error" : ""}`} role={authFeedback.variant === "error" ? "alert" : "status"}>
                {authFeedback.message}
              </p>
            ) : null}

            {mfaRequired ? (
              <form className="auth-overlay__form" onSubmit={handleMFASubmit}>
                <div className="stack stack--2xs">
                  <h3>Finish with multi-factor verification</h3>
                  <p className="muted">Enter a current authenticator code or one of your recovery codes to continue.</p>
                </div>

                {mfaEnrollment && (
                  <div className="auth-overlay__enrollment surface">
                    <div className="stack stack--2xs">
                      <strong>Set up your authenticator</strong>
                      <p className="muted">Save these details before you verify, especially if this is your first MFA enrollment.</p>
                    </div>
                    <label className="auth-overlay__field">
                      <span>Secret</span>
                      <input type="text" readOnly value={mfaEnrollment.secret} />
                    </label>
                    <label className="auth-overlay__field">
                      <span>Authenticator URL</span>
                      <input type="text" readOnly value={mfaEnrollment.otpauthUrl} />
                    </label>
                    {mfaEnrollment.recoveryCodes.length > 0 && (
                      <div className="stack stack--2xs">
                        <span>Recovery codes</span>
                        <ul className="auth-overlay__recovery-list">
                          {mfaEnrollment.recoveryCodes.map((code) => (
                            <li key={code}>{code}</li>
                          ))}
                        </ul>
                      </div>
                    )}
                  </div>
                )}

                <label className="auth-overlay__field">
                  <span>Verification code</span>
                  <input
                    ref={initialFocusRef}
                    type="text"
                    inputMode="numeric"
                    autoComplete="one-time-code"
                    value={mfaCode}
                    onChange={(event) => setMFACode(event.target.value)}
                    placeholder="123456"
                    required
                  />
                </label>

                <div className="auth-overlay__actions">
                  <button type="submit" className="accent-button" disabled={loading}>
                    {loading ? "Verifying..." : "Verify"}
                  </button>
                  <button type="button" className="ghost-button" onClick={closeAuthDialog}>
                    Cancel
                  </button>
                </div>
              </form>
            ) : authMode === "signup" ? (
              <form className="auth-overlay__form" onSubmit={handleSignUpSubmit}>
                <div className="stack stack--2xs">
                  <h3>Create your viewer account</h3>
                  <p className="muted">Join without leaving the stream discovery flow.</p>
                </div>

                <label className="auth-overlay__field">
                  <span>Display name</span>
                  <input
                    ref={initialFocusRef}
                    type="text"
                    autoComplete="nickname"
                    value={signUpDisplayName}
                    onChange={(event) => setSignUpDisplayName(event.target.value)}
                    required
                  />
                </label>

                <label className="auth-overlay__field">
                  <span>Email</span>
                  <input
                    type="email"
                    autoComplete="email"
                    value={signUpEmail}
                    onChange={(event) => setSignUpEmail(event.target.value)}
                    required
                  />
                </label>

                <label className="auth-overlay__field">
                  <span>Password</span>
                  <input
                    type="password"
                    autoComplete="new-password"
                    value={signUpPassword}
                    onChange={(event) => setSignUpPassword(event.target.value)}
                    required
                  />
                </label>

                <div className="auth-overlay__actions">
                  <button type="submit" className="accent-button" disabled={loading}>
                    {loading ? "Creating account..." : "Create account"}
                  </button>
                  <button type="button" className="ghost-button" onClick={() => setAuthMode("signin")}>
                    I already have an account
                  </button>
                </div>
              </form>
            ) : (
              <form className="auth-overlay__form" onSubmit={handleSignInSubmit}>
                <div className="stack stack--2xs">
                  <h3>Sign in without losing your place</h3>
                  <p className="muted">Your stream, category, and browse context stay right here while you authenticate.</p>
                </div>

                <label className="auth-overlay__field">
                  <span>Email</span>
                  <input
                    ref={initialFocusRef}
                    type="email"
                    autoComplete="email"
                    value={signInEmail}
                    onChange={(event) => setSignInEmail(event.target.value)}
                    required
                  />
                </label>

                <label className="auth-overlay__field">
                  <span>Password</span>
                  <input
                    type="password"
                    autoComplete="current-password"
                    value={signInPassword}
                    onChange={(event) => setSignInPassword(event.target.value)}
                    required
                  />
                </label>

                <div className="auth-overlay__actions">
                  <button type="submit" className="accent-button" disabled={loading}>
                    {loading ? "Signing in..." : "Sign in"}
                  </button>
                  {allowSelfSignup && (
                    <button type="button" className="ghost-button" onClick={() => setAuthMode("signup")}>
                      Need an account?
                    </button>
                  )}
                </div>
              </form>
            )}
          </>
        )}
      </section>
    </div>
  );
}
