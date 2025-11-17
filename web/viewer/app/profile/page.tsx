"use client";

import Link from "next/link";
import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { useAuth } from "../../hooks/useAuth";
import type { ProfileView } from "../../lib/viewer-api";
import { fetchProfile, updateProfile } from "../../lib/viewer-api";

type FormState = {
  displayName: string;
  email: string;
  bio: string;
  avatarUrl: string;
  bannerUrl: string;
};

const defaultFormState: FormState = {
  displayName: "",
  email: "",
  bio: "",
  avatarUrl: "",
  bannerUrl: "",
};

export default function ProfilePage() {
  const { user, loading: authLoading, error: authError, refresh } = useAuth();
  const [profile, setProfile] = useState<ProfileView | undefined>();
  const [formState, setFormState] = useState<FormState>(defaultFormState);
  const [loadingProfile, setLoadingProfile] = useState(false);
  const [profileError, setProfileError] = useState<string | undefined>();
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | undefined>();
  const [successMessage, setSuccessMessage] = useState<string | undefined>();

  const loadProfile = useCallback(async () => {
    if (!user) {
      setProfile(undefined);
      setFormState(defaultFormState);
      return;
    }
    try {
      setLoadingProfile(true);
      setProfileError(undefined);
      const data = await fetchProfile(user.id);
      setProfile(data);
      setFormState({
        displayName: data.displayName ?? user.displayName ?? "",
        email: user.email ?? "",
        bio: data.bio ?? "",
        avatarUrl: data.avatarUrl ?? "",
        bannerUrl: data.bannerUrl ?? "",
      });
    } catch (err) {
      setProfile(undefined);
      setProfileError(err instanceof Error ? err.message : "Unable to load profile");
    } finally {
      setLoadingProfile(false);
    }
  }, [user]);

  useEffect(() => {
    void loadProfile();
  }, [loadProfile]);

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault();
    if (!user) {
      return;
    }
    try {
      setSaving(true);
      setSaveError(undefined);
      setSuccessMessage(undefined);
      const updated = await updateProfile(user.id, {
        displayName: formState.displayName,
        email: formState.email,
        bio: formState.bio,
        avatarUrl: formState.avatarUrl,
        bannerUrl: formState.bannerUrl,
      });
      setProfile(updated);
      setSuccessMessage("Profile saved");
      await refresh();
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : "Unable to save profile");
    } finally {
      setSaving(false);
    }
  };

  const handleReset = () => {
    if (!profile) {
      setFormState({
        ...defaultFormState,
        displayName: user.displayName,
        email: user.email ?? "",
      });
      return;
    }
    setFormState({
      displayName: profile.displayName ?? user.displayName ?? "",
      email: user.email ?? "",
      bio: profile.bio ?? "",
      avatarUrl: profile.avatarUrl ?? "",
      bannerUrl: profile.bannerUrl ?? "",
    });
    setSaveError(undefined);
    setSuccessMessage(undefined);
  };

  const hasProfileContent = useMemo(() => {
    return Boolean(formState.bio.trim() || formState.avatarUrl.trim() || formState.bannerUrl.trim());
  }, [formState.bio, formState.avatarUrl, formState.bannerUrl]);

  const avatarGlyph = useMemo(() => {
    if (formState.avatarUrl.trim()) {
      return (
        <img
          src={formState.avatarUrl}
          alt="Profile avatar"
          style={{ width: "4rem", height: "4rem", borderRadius: "999px", objectFit: "cover" }}
        />
      );
    }
    const initial = (profile?.displayName ?? user?.displayName ?? "?").slice(0, 1).toUpperCase();
    return (
      <span
        aria-hidden
        style={{
          display: "inline-flex",
          alignItems: "center",
          justifyContent: "center",
          width: "4rem",
          height: "4rem",
          borderRadius: "999px",
          background: "var(--surface-3)",
          color: "var(--text-muted)",
          fontWeight: 700,
        }}
      >
        {initial || "?"}
      </span>
    );
  }, [formState.avatarUrl, profile?.displayName, user?.displayName]);

  return (
    <div className="container stack" style={{ paddingTop: "2rem", paddingBottom: "4rem", gap: "1.5rem" }}>
      <header className="stack">
        <h1>Profile</h1>
        <p className="muted">Update how others see you across BitRiver Live.</p>
      </header>

      {authError && (
        <div className="surface" role="alert">
          {authError}
        </div>
      )}

      {authLoading ? (
        <section className="surface">Loading your account…</section>
      ) : !user ? (
        <section className="surface stack">
          <h2>Sign in to manage your profile</h2>
          <p className="muted">Your avatar, banner, and bio will appear on your channel cards once you&apos;re signed in.</p>
          <div>
            <Link href="/" className="primary-button">
              Return home
            </Link>
          </div>
        </section>
      ) : (
        <div className="stack" style={{ gap: "1.5rem" }}>
          {profileError && (
            <section className="surface stack" role="alert">
              <div className="section-heading">
                <div>
                  <h2>Unable to load profile</h2>
                  <p className="muted">{profileError}</p>
                </div>
                <button type="button" className="secondary-button" onClick={() => { void loadProfile(); }}>
                  Try again
                </button>
              </div>
            </section>
          )}

          {loadingProfile ? (
            <section className="surface">Loading your profile…</section>
          ) : (
            <>
              <section className="surface stack" style={{ overflow: "hidden" }}>
                <div
                  aria-label="Profile banner"
                  style={{
                    height: "10rem",
                    width: "100%",
                    backgroundColor: hasProfileContent ? "var(--surface-3)" : "var(--surface-2)",
                    backgroundImage: formState.bannerUrl ? `url(${formState.bannerUrl})` : undefined,
                    backgroundSize: "cover",
                    backgroundPosition: "center",
                  }}
                />
                <div style={{ display: "flex", gap: "1rem", alignItems: "flex-start" }}>
                  {avatarGlyph}
                  <div className="stack" style={{ gap: "0.25rem" }}>
                    <div>
                      <p className="muted" style={{ margin: 0 }}>
                        Signed in as
                      </p>
                      <h2 style={{ margin: 0 }}>{profile?.displayName ?? user.displayName}</h2>
                    </div>
                    <p className={formState.bio.trim() ? "" : "muted"}>
                      {formState.bio.trim() ? formState.bio : "Add a short bio so viewers know what to expect."}
                    </p>
                    {!hasProfileContent && (
                      <p className="muted">Start by adding a banner, avatar, or short bio below.</p>
                    )}
                  </div>
                </div>
              </section>

              <form className="surface stack" onSubmit={handleSubmit}>
                <div className="stack" style={{ gap: "1.5rem" }}>
                  <div className="stack" style={{ gap: "0.75rem" }}>
                    <div>
                      <h2 style={{ margin: 0 }}>Account</h2>
                      <p className="muted" style={{ margin: 0 }}>
                        Keep your display name and contact email up to date so viewers and notifications reach you.
                      </p>
                    </div>

                    <div className="stack" style={{ gap: "0.25rem" }}>
                      <label htmlFor="displayName">Display name</label>
                      <input
                        id="displayName"
                        name="displayName"
                        type="text"
                        required
                        placeholder="How viewers see you"
                        value={formState.displayName}
                        onChange={(event) => setFormState((prev) => ({ ...prev, displayName: event.target.value }))}
                      />
                      <p className="muted">Shown on your channel cards, chat messages, and profile.</p>
                    </div>

                    <div className="stack" style={{ gap: "0.25rem" }}>
                      <label htmlFor="email">Email</label>
                      <input
                        id="email"
                        name="email"
                        type="email"
                        required
                        placeholder="you@example.com"
                        value={formState.email}
                        onChange={(event) => setFormState((prev) => ({ ...prev, email: event.target.value }))}
                      />
                      <p className="muted">We&apos;ll use this for updates, notifications, and account recovery.</p>
                    </div>
                  </div>

                  <div className="stack" style={{ gap: "0.75rem" }}>
                    <div>
                      <h2 style={{ margin: 0 }}>Profile visuals</h2>
                      <p className="muted" style={{ margin: 0 }}>
                        Personalize your channel preview with images and a short bio.
                      </p>
                    </div>

                    <div className="stack" style={{ gap: "0.25rem" }}>
                      <label htmlFor="avatarUrl">Avatar URL</label>
                      <input
                        id="avatarUrl"
                        name="avatarUrl"
                        type="url"
                        placeholder="https://example.com/avatar.png"
                        value={formState.avatarUrl}
                        onChange={(event) => setFormState((prev) => ({ ...prev, avatarUrl: event.target.value }))}
                      />
                      <p className="muted">Use a square image for best results.</p>
                    </div>

                    <div className="stack" style={{ gap: "0.25rem" }}>
                      <label htmlFor="bannerUrl">Banner URL</label>
                      <input
                        id="bannerUrl"
                        name="bannerUrl"
                        type="url"
                        placeholder="https://example.com/banner.jpg"
                        value={formState.bannerUrl}
                        onChange={(event) => setFormState((prev) => ({ ...prev, bannerUrl: event.target.value }))}
                      />
                      <p className="muted">Wide images shine here. Leave blank for a neutral background.</p>
                    </div>

                    <div className="stack" style={{ gap: "0.25rem" }}>
                      <label htmlFor="bio">Bio</label>
                      <textarea
                        id="bio"
                        name="bio"
                        rows={4}
                        placeholder="Tell viewers about your streams, schedule, or community."
                        value={formState.bio}
                        onChange={(event) => setFormState((prev) => ({ ...prev, bio: event.target.value }))}
                      />
                    </div>
                  </div>
                </div>

                {saveError && (
                  <p className="error" role="alert">
                    {saveError}
                  </p>
                )}
                {successMessage && <p className="success">{successMessage}</p>}

                <div style={{ display: "flex", gap: "0.75rem", flexWrap: "wrap" }}>
                  <button type="submit" className="primary-button" disabled={saving}>
                    {saving ? "Saving…" : "Save profile"}
                  </button>
                  <button type="button" className="secondary-button" onClick={handleReset} disabled={saving}>
                    Reset changes
                  </button>
                </div>
              </form>
            </>
          )}
        </div>
      )}
    </div>
  );
}
