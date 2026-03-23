"use client";

import Image from "next/image";
import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { Button } from "../../components/ui/Button";
import { Card, CardBody, CardHeader } from "../../components/ui/Card";
import { InlineAlert } from "../../components/ui/InlineAlert";
import { useAuth } from "../../hooks/useAuth";
import type { ProfileView, SocialLink } from "../../lib/viewer-api";
import { fetchProfile, updateProfile } from "../../lib/viewer-api";

type FormState = {
  displayName: string;
  email: string;
  bio: string;
  avatarUrl: string;
  bannerUrl: string;
  socialLinks: SocialLink[];
};

const defaultFormState: FormState = {
  displayName: "",
  email: "",
  bio: "",
  avatarUrl: "",
  bannerUrl: "",
  socialLinks: [],
};

export default function ProfilePage() {
  const { user, loading: authLoading, error: authError, signIn } = useAuth();
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
        socialLinks: data.socialLinks ?? [],
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
        socialLinks: formState.socialLinks,
      });
      setProfile(updated);
      setSuccessMessage("Profile saved");
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : "Unable to save profile");
    } finally {
      setSaving(false);
    }
  };

  const handleReset = () => {
    if (!user) {
      setFormState(defaultFormState);
      setSaveError(undefined);
      setSuccessMessage(undefined);
      return;
    }

    if (!profile) {
      setFormState({
        ...defaultFormState,
        displayName: user.displayName ?? "",
        email: user.email ?? "",
      });
      setSaveError(undefined);
      setSuccessMessage(undefined);
      return;
    }

    setFormState({
      displayName: profile.displayName ?? user.displayName ?? "",
      email: user.email ?? "",
      bio: profile.bio ?? "",
      avatarUrl: profile.avatarUrl ?? "",
      bannerUrl: profile.bannerUrl ?? "",
      socialLinks: profile.socialLinks ?? [],
    });
    setSaveError(undefined);
    setSuccessMessage(undefined);
  };

  const hasProfileContent = useMemo(() => {
    const hasSocialLinks = formState.socialLinks.some((link) => link.url.trim());
    return Boolean(formState.bio.trim() || formState.avatarUrl.trim() || formState.bannerUrl.trim() || hasSocialLinks);
  }, [formState.avatarUrl, formState.bannerUrl, formState.bio, formState.socialLinks]);

  const completedSections = useMemo(() => {
    let count = 0;
    if (formState.displayName.trim()) count += 1;
    if (formState.bio.trim()) count += 1;
    if (formState.avatarUrl.trim()) count += 1;
    if (formState.bannerUrl.trim()) count += 1;
    if (formState.socialLinks.some((link) => link.url.trim())) count += 1;
    return count;
  }, [formState.avatarUrl, formState.bannerUrl, formState.bio, formState.displayName, formState.socialLinks]);

  const handleSocialLinkChange = (index: number, field: keyof SocialLink, value: string) => {
    setFormState((prev) => ({
      ...prev,
      socialLinks: prev.socialLinks.map((link, linkIndex) => (linkIndex === index ? { ...link, [field]: value } : link)),
    }));
  };

  const handleAddSocialLink = () => {
    setFormState((prev) => ({
      ...prev,
      socialLinks: [...prev.socialLinks, { platform: "", url: "" }],
    }));
  };

  const handleRemoveSocialLink = (index: number) => {
    setFormState((prev) => ({
      ...prev,
      socialLinks: prev.socialLinks.filter((_, linkIndex) => linkIndex !== index),
    }));
  };

  const avatarGlyph = useMemo(() => {
    if (formState.avatarUrl.trim()) {
      return (
        <div className="profile-avatar">
          <Image src={formState.avatarUrl} alt="Profile avatar" width={72} height={72} sizes="72px" />
        </div>
      );
    }

    const initial = (profile?.displayName ?? user?.displayName ?? "?").slice(0, 1).toUpperCase();
    return <span className="profile-avatar profile-avatar--fallback">{initial || "?"}</span>;
  }, [formState.avatarUrl, profile?.displayName, user?.displayName]);

  const connectedSocialCount = formState.socialLinks.filter((link) => link.url.trim()).length;

  return (
    <div className="workspace-page workspace-page--narrow">
      <section className="workspace-hero">
        <div className="workspace-hero__copy">
          <span className="page-eyebrow">Profile</span>
          <h1>Shape how your identity appears across BitRiver Live</h1>
          <p className="muted">
            Keep your public bio, visuals, and contact details aligned so channel cards and profile surfaces feel consistent.
          </p>
        </div>
        <div className="workspace-summary-grid">
          <article className="summary-card">
            <span className="summary-card__label">Profile progress</span>
            <strong className="summary-card__value">{completedSections}/5</strong>
            <p className="muted">Core profile fields completed</p>
          </article>
          <article className="summary-card">
            <span className="summary-card__label">Social links</span>
            <strong className="summary-card__value">{connectedSocialCount}</strong>
            <p className="muted">Connected destinations for viewers</p>
          </article>
          <article className="summary-card">
            <span className="summary-card__label">Account</span>
            <strong className="summary-card__value">{user?.email ?? "Guest"}</strong>
            <p className="muted">Signed-in contact currently tied to the profile</p>
          </article>
        </div>
      </section>

      {authError ? (
        <Card className="workspace-card" role="alert">
          <InlineAlert>{authError}</InlineAlert>
        </Card>
      ) : null}

      {authLoading ? (
        <Card className="workspace-card">Loading your account...</Card>
      ) : !user ? (
        <Card className="workspace-card">
          <CardHeader className="workspace-card__header">
            <h2>Sign in to manage your profile</h2>
            <p className="muted">Your avatar, banner, and bio will appear on your channel cards once you are signed in.</p>
          </CardHeader>
          <div className="workspace-card__actions">
            <Button onClick={() => void signIn()}>Sign in</Button>
          </div>
        </Card>
      ) : (
        <div className="profile-layout">
          {profileError ? (
            <Card className="workspace-card" role="alert">
              <CardHeader className="workspace-card__header">
                <h2>Unable to load profile</h2>
                <p className="muted">{profileError}</p>
              </CardHeader>
              <div className="workspace-card__actions">
                <Button
                  variant="secondary"
                  onClick={() => {
                    void loadProfile();
                  }}
                >
                  Try again
                </Button>
              </div>
            </Card>
          ) : null}

          {loadingProfile ? (
            <Card className="workspace-card">Loading your profile...</Card>
          ) : (
            <>
              <Card className="workspace-card profile-hero">
                <div
                  aria-label="Profile banner"
                  className="profile-banner"
                  style={{
                    backgroundColor: hasProfileContent ? "var(--surface-3)" : "var(--surface-2)",
                    backgroundImage: formState.bannerUrl ? `url(${formState.bannerUrl})` : undefined,
                  }}
                />
                <div className="profile-identity">
                  {avatarGlyph}
                  <div className="stack">
                    <div className="stack stack--2xs">
                      <p className="muted">Signed in as</p>
                      <h2>{profile?.displayName ?? user.displayName}</h2>
                    </div>
                    <p className={formState.bio.trim() ? "" : "muted"}>
                      {formState.bio.trim() ? formState.bio : "Add a short bio so viewers know what to expect."}
                    </p>
                    {!hasProfileContent ? <p className="muted">Start by adding a banner, avatar, or short bio below.</p> : null}
                  </div>
                </div>
              </Card>

              <form className="profile-form" onSubmit={handleSubmit}>
                <div className="profile-form__grid">
                  <Card className="workspace-card">
                    <CardHeader className="workspace-card__header">
                      <h2>Account</h2>
                      <p className="muted">Keep your display name and contact email current so viewers and notifications reach you.</p>
                    </CardHeader>
                    <CardBody className="workspace-card__header">
                      <div className="input-stack">
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
                      <div className="input-stack">
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
                        <p className="muted">We will use this for updates, notifications, and account recovery.</p>
                      </div>
                    </CardBody>
                  </Card>

                  <Card className="workspace-card">
                    <CardHeader className="workspace-card__header">
                      <h2>Profile visuals</h2>
                      <p className="muted">Personalize your channel preview with images and a short bio.</p>
                    </CardHeader>
                    <CardBody className="workspace-card__header">
                      <div className="input-stack">
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
                      <div className="input-stack">
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
                      <div className="input-stack">
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
                    </CardBody>
                  </Card>

                  <Card className="workspace-card workspace-grid__full">
                    <CardHeader className="workspace-card__header">
                      <h2>Social links</h2>
                      <p className="muted">Share where viewers can follow you outside BitRiver Live.</p>
                    </CardHeader>
                    <CardBody className="workspace-card__header">
                      {formState.socialLinks.length === 0 ? <p className="muted">Add platforms and URLs to feature on your profile.</p> : null}
                      <div className="profile-social-list">
                        {formState.socialLinks.map((link, index) => (
                          <div key={`social-${index}`} className="profile-social-card">
                            <div className="input-stack">
                              <label htmlFor={`social-platform-${index}`}>Platform</label>
                              <input
                                id={`social-platform-${index}`}
                                name={`social-platform-${index}`}
                                type="text"
                                placeholder="Platform or label"
                                value={link.platform}
                                onChange={(event) => handleSocialLinkChange(index, "platform", event.target.value)}
                              />
                            </div>
                            <div className="input-stack">
                              <label htmlFor={`social-url-${index}`}>Link</label>
                              <input
                                id={`social-url-${index}`}
                                name={`social-url-${index}`}
                                type="url"
                                placeholder="https://example.com/you"
                                value={link.url}
                                onChange={(event) => handleSocialLinkChange(index, "url", event.target.value)}
                              />
                            </div>
                            <div className="workspace-card__actions">
                              <button type="button" className="ghost-button" onClick={() => handleRemoveSocialLink(index)}>
                                Remove link
                              </button>
                            </div>
                          </div>
                        ))}
                      </div>
                      <div className="workspace-card__actions">
                        <button type="button" className="secondary-button" onClick={handleAddSocialLink}>
                          Add social link
                        </button>
                      </div>
                    </CardBody>
                  </Card>
                </div>

                {saveError ? (
                  <p className="error" role="alert">
                    {saveError}
                  </p>
                ) : null}
                {successMessage ? <p className="success">{successMessage}</p> : null}

                <div className="profile-actions">
                  <Button type="submit" disabled={saving}>
                    {saving ? "Saving..." : "Save profile"}
                  </Button>
                  <Button type="button" variant="secondary" onClick={handleReset} disabled={saving}>
                    Reset changes
                  </Button>
                </div>
              </form>
            </>
          )}
        </div>
      )}
    </div>
  );
}
