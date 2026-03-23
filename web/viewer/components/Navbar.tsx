"use client";

import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { FormEvent, useEffect, useMemo, useRef, useState } from "react";
import { useAuth } from "../hooks/useAuth";
import { appendHash, joinConfiguredPath, resolveSignupUrl } from "../lib/auth-links";
import { deriveQuickLinks, getNavigationAudience, getVisibleNavigationItems } from "../lib/navigation";
import { fetchManagedChannels } from "../lib/viewer-api";

const LOCAL_SETUP_DOCS_ROUTE = "/getting-started";
const THEME_STORAGE_KEY = "viewer-theme";
const LIGHT_THEME_QUERY = "(prefers-color-scheme: light)";
const MOBILE_NAV_QUERY = "(max-width: 1080px)";
const DESKTOP_NAV_QUERY = "(min-width: 1081px)";
const FOCUSABLE_SELECTOR =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

const isLocalhostUrl = (rawUrl?: string) => {
  const value = rawUrl?.trim();
  if (!value) {
    return false;
  }

  try {
    const parsed = new URL(value);
    return parsed.hostname === "localhost" || parsed.hostname === "127.0.0.1" || parsed.hostname === "::1";
  } catch {
    return false;
  }
};

export function Navbar() {
  const { user, signIn, signOut } = useAuth();
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const searchParamQuery = searchParams?.get("q") ?? "";
  const isAdmin = Boolean(user?.roles?.includes("admin"));
  const isCreator = Boolean(user?.roles?.includes("creator"));
  const canAccessCreatorTools = isAdmin || isCreator;
  const [theme, setTheme] = useState<"dark" | "light">("dark");
  const [hasExplicitThemePreference, setHasExplicitThemePreference] = useState(false);
  const [managedChannelId, setManagedChannelId] = useState<string | undefined>();
  const [searchQuery, setSearchQuery] = useState(searchParamQuery);
  const [menuOpen, setMenuOpen] = useState(false);
  const [isMobileDrawerPresentation, setIsMobileDrawerPresentation] = useState(false);
  const [userMenuOpen, setUserMenuOpen] = useState(false);
  const menuToggleRef = useRef<HTMLButtonElement | null>(null);
  const navDrawerRef = useRef<HTMLDivElement | null>(null);
  const avatarButtonRef = useRef<HTMLButtonElement | null>(null);
  const avatarMenuRef = useRef<HTMLDivElement | null>(null);
  const previousMenuOpenRef = useRef(false);

  const normalizedPathname = (() => {
    const current = pathname ?? "/";
    if (current === "/") {
      return "/";
    }
    if (current.startsWith("/viewer")) {
      const trimmed = current.replace(/^\/viewer/, "");
      return trimmed.length === 0 ? "/" : trimmed;
    }
    return current;
  })();
  const canonicalPath = normalizedPathname.startsWith("/") ? normalizedPathname : `/${normalizedPathname}`;
  const navigationAudience = useMemo(
    () => getNavigationAudience({ isAuthenticated: Boolean(user), roles: user?.roles }),
    [user],
  );
  const navItems = useMemo(() => getVisibleNavigationItems(navigationAudience), [navigationAudience]);
  const quickLinks = useMemo(() => deriveQuickLinks(navigationAudience, navItems), [navigationAudience, navItems]);
  const configuredSignupUrl = process.env.NEXT_PUBLIC_SIGNUP_URL?.trim();
  const shouldShowLocalSetupBanner =
    process.env.NODE_ENV !== "production" &&
    (isLocalhostUrl(process.env.NEXT_PUBLIC_VIEWER_URL) || isLocalhostUrl(process.env.NEXT_PUBLIC_API_BASE_URL));
  const signupUrl = useMemo(() => resolveSignupUrl(configuredSignupUrl), [configuredSignupUrl]);
  const adminUrl = useMemo(() => joinConfiguredPath(process.env.NEXT_PUBLIC_API_BASE_URL, "/admin"), []);
  const studioHref = managedChannelId ? `/creator/live/${managedChannelId}` : "/creator/getting-started";

  const isRouteActive = (href: string) => {
    if (href === "/") {
      return canonicalPath === "/";
    }
    return canonicalPath === href || canonicalPath.startsWith(`${href}/`);
  };

  useEffect(() => {
    setSearchQuery((prev) => (prev === searchParamQuery ? prev : searchParamQuery));
  }, [searchParamQuery]);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }

    const storedTheme = window.localStorage.getItem(THEME_STORAGE_KEY);
    if (storedTheme === "light" || storedTheme === "dark") {
      setTheme(storedTheme);
      setHasExplicitThemePreference(true);
      return;
    }

    if (!window.matchMedia) {
      return;
    }

    const query = window.matchMedia(LIGHT_THEME_QUERY);
    setTheme(query.matches ? "light" : "dark");
  }, []);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    if (!window.matchMedia || hasExplicitThemePreference) {
      return;
    }

    const query = window.matchMedia(LIGHT_THEME_QUERY);
    const handler = (event: MediaQueryListEvent) => setTheme(event.matches ? "light" : "dark");
    query.addEventListener("change", handler);
    return () => {
      query.removeEventListener("change", handler);
    };
  }, [hasExplicitThemePreference]);

  useEffect(() => {
    if (typeof window === "undefined" || !window.matchMedia) {
      return;
    }

    const query = window.matchMedia(MOBILE_NAV_QUERY);
    const syncMobilePresentation = (matches: boolean) => setIsMobileDrawerPresentation(matches);
    syncMobilePresentation(query.matches);

    const handleChange = (event: MediaQueryListEvent) => {
      syncMobilePresentation(event.matches);
    };

    query.addEventListener("change", handleChange);
    return () => {
      query.removeEventListener("change", handleChange);
    };
  }, []);

  useEffect(() => {
    if (typeof window === "undefined" || !window.matchMedia) {
      return;
    }

    const query = window.matchMedia(DESKTOP_NAV_QUERY);
    if (query.matches) {
      setMenuOpen(false);
    }

    const handler = (event: MediaQueryListEvent) => {
      if (event.matches) {
        setMenuOpen(false);
      }
    };

    query.addEventListener("change", handler);
    return () => {
      query.removeEventListener("change", handler);
    };
  }, []);

  useEffect(() => {
    if (typeof document === "undefined") {
      return;
    }

    if (theme === "light") {
      document.body.setAttribute("data-theme", "light");
    } else {
      document.body.removeAttribute("data-theme");
    }
  }, [theme]);

  useEffect(() => {
    if (typeof document === "undefined") {
      return;
    }

    const wasOpen = previousMenuOpenRef.current;
    const isModalDrawerOpen = menuOpen && isMobileDrawerPresentation;
    previousMenuOpenRef.current = menuOpen;

    if (!isModalDrawerOpen) {
      if (wasOpen && !menuOpen) {
        menuToggleRef.current?.focus();
      }
      return;
    }

    const navDrawer = navDrawerRef.current;
    if (!navDrawer) {
      return;
    }

    const originalOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";

    const focusable = Array.from(navDrawer.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR));
    focusable[0]?.focus();

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        setMenuOpen(false);
        return;
      }
      if (event.key !== "Tab") {
        return;
      }

      const elements = Array.from(navDrawer.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR));
      if (elements.length === 0) {
        return;
      }

      const first = elements[0];
      const last = elements[elements.length - 1];
      const activeElement = document.activeElement;

      if (event.shiftKey) {
        if (activeElement === first || !navDrawer.contains(activeElement)) {
          event.preventDefault();
          last.focus();
        }
        return;
      }

      if (activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    };

    document.addEventListener("keydown", handleKeyDown);

    return () => {
      document.removeEventListener("keydown", handleKeyDown);
      document.body.style.overflow = originalOverflow;
    };
  }, [menuOpen, isMobileDrawerPresentation]);

  useEffect(() => {
    let cancelled = false;
    if (!user) {
      setManagedChannelId(undefined);
      return () => {
        cancelled = true;
      };
    }

    const hasManagementRole = user.roles.includes("creator") || isAdmin;
    if (!hasManagementRole) {
      setManagedChannelId(undefined);
      return () => {
        cancelled = true;
      };
    }

    const loadChannels = async () => {
      try {
        const channels = await fetchManagedChannels();
        if (!cancelled) {
          setManagedChannelId(channels[0]?.id);
        }
      } catch (err) {
        if (!cancelled) {
          setManagedChannelId(undefined);
          console.error("Unable to load managed channels", err);
        }
      }
    };

    void loadChannels();

    return () => {
      cancelled = true;
    };
  }, [user, isAdmin]);

  useEffect(() => {
    setMenuOpen(false);
    setUserMenuOpen(false);
  }, [user]);

  useEffect(() => {
    if (!userMenuOpen || typeof document === "undefined") {
      return;
    }

    let closed = false;
    const closeMenuWithFocusRestore = () => {
      if (closed) {
        return;
      }
      closed = true;
      setUserMenuOpen(false);
      avatarButtonRef.current?.focus();
    };

    const handleOutsideInteraction = (event: PointerEvent | MouseEvent) => {
      const target = event.target;
      if (!(target instanceof Node)) {
        return;
      }
      if (avatarButtonRef.current?.contains(target) || avatarMenuRef.current?.contains(target)) {
        return;
      }
      closeMenuWithFocusRestore();
    };

    const handleEscapeKey = (event: KeyboardEvent) => {
      if (event.key !== "Escape") {
        return;
      }
      event.preventDefault();
      closeMenuWithFocusRestore();
    };

    document.addEventListener("pointerdown", handleOutsideInteraction);
    document.addEventListener("click", handleOutsideInteraction);
    document.addEventListener("keydown", handleEscapeKey);

    return () => {
      document.removeEventListener("pointerdown", handleOutsideInteraction);
      document.removeEventListener("click", handleOutsideInteraction);
      document.removeEventListener("keydown", handleEscapeKey);
    };
  }, [userMenuOpen]);

  const closeMenu = () => {
    setMenuOpen(false);
  };

  const buildRedirectTarget = () => {
    if (typeof window === "undefined") {
      return "/";
    }
    return `${window.location.pathname}${window.location.search}${window.location.hash}`;
  };

  const handleSignIn = () => {
    closeMenu();
    void signIn(buildRedirectTarget());
  };

  const handleJoin = () => {
    closeMenu();
    if (!signupUrl) {
      handleSignIn();
      return;
    }
    if (typeof window === "undefined") {
      return;
    }

    const url = new URL(appendHash(signupUrl, "signup-card"), window.location.origin);
    if (!url.searchParams.has("next")) {
      url.searchParams.set("next", buildRedirectTarget());
    }
    window.location.href = url.toString();
  };

  const handleSearch = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const trimmed = searchQuery.trim();
    await router.push(trimmed ? `/browse?q=${encodeURIComponent(trimmed)}` : "/browse");
    closeMenu();
  };

  const avatarGlyph = useMemo(() => {
    const initial = user?.displayName?.trim().charAt(0).toUpperCase();
    return initial || "U";
  }, [user?.displayName]);

  return (
    <header className="navbar">
      {shouldShowLocalSetupBanner && (
        <div className="local-setup-banner" role="status">
          <span>Running in local setup mode. Before going public, configure your domain and CORS settings.</span>{" "}
          <Link href={LOCAL_SETUP_DOCS_ROUTE} className="local-setup-banner__link" onClick={closeMenu}>
            Setup guide
          </Link>
        </div>
      )}

      <div className="navbar-inner">
        <div className="navbar-main">
          <div className="navbar-branding">
            <button
              ref={menuToggleRef}
              className="nav-toggle"
              type="button"
              aria-expanded={menuOpen}
              aria-controls="viewer-nav-menu"
              aria-label={menuOpen ? "Close navigation menu" : "Open navigation menu"}
              onClick={() => setMenuOpen((prev) => !prev)}
            >
              {menuOpen ? "Close" : "Menu"}
            </button>

            <Link href="/" aria-label="BitRiver Live home" className="navbar-logo" onClick={closeMenu}>
              <span className="navbar-logo__icon" aria-hidden="true">
                BR
              </span>
              <span className="navbar-logo__copy">
                <span className="navbar-logo__text">BitRiver Live</span>
                <span className="navbar-logo__meta">Self-hosted creator network</span>
              </span>
            </Link>
          </div>

          <nav className="nav-tabs" aria-label="Primary navigation">
            {navItems.map((item) => {
              const active = isRouteActive(item.href);
              return (
                <Link
                  key={item.href}
                  href={item.href}
                  className={`nav-tab${active ? " nav-tab--active" : ""}`}
                  aria-current={active ? "page" : undefined}
                  onClick={closeMenu}
                >
                  {item.label}
                </Link>
              );
            })}
          </nav>

          <div className="navbar-center">
            <form className="nav-search nav-search--inline" role="search" onSubmit={handleSearch}>
              <label className="sr-only" htmlFor="navbar-search">
                Search for channels or categories
              </label>
              <input
                id="navbar-search"
                className="nav-search__input"
                type="search"
                placeholder="Search channels, creators, or tags"
                value={searchQuery}
                onChange={(event) => setSearchQuery(event.target.value)}
              />
              <button type="submit" className="nav-search__submit">
                Search
              </button>
            </form>
          </div>

          <div className="navbar-right">
            {isAdmin && (
              <a href={adminUrl} className="nav-cta nav-cta--secondary nav-utility-link" onClick={closeMenu}>
                Control center
              </a>
            )}
            {canAccessCreatorTools && (
              <Link href={studioHref} className="nav-cta nav-cta--primary" onClick={closeMenu}>
                {managedChannelId ? "Studio" : "Creator studio"}
              </Link>
            )}

            <div className="nav-icon-group" role="group" aria-label="Account and preferences">
              <button
                className="icon-button icon-button--text"
                type="button"
                onClick={() => {
                  const nextTheme = theme === "light" ? "dark" : "light";
                  setTheme(nextTheme);
                  setHasExplicitThemePreference(true);
                  if (typeof window !== "undefined") {
                    window.localStorage.setItem(THEME_STORAGE_KEY, nextTheme);
                  }
                }}
                aria-label={`Switch to ${theme === "light" ? "dark" : "light"} theme`}
              >
                {theme === "light" ? "Dark" : "Light"}
              </button>

              {user ? (
                <div className="avatar-menu" aria-label="Account menu">
                  <button
                    type="button"
                    className="avatar-button"
                    aria-label="Open account menu"
                    aria-expanded={userMenuOpen}
                    aria-controls="viewer-user-menu"
                    ref={avatarButtonRef}
                    onClick={() => setUserMenuOpen((prev) => !prev)}
                  >
                    {avatarGlyph}
                  </button>
                  <div
                    id="viewer-user-menu"
                    ref={avatarMenuRef}
                    className={`avatar-menu__items${userMenuOpen ? " avatar-menu__items--open" : ""}`}
                  >
                    <div className="avatar-menu__header">
                      <span className="avatar-menu__eyebrow">Signed in as</span>
                      <span className="avatar-menu__name">{user.displayName}</span>
                    </div>
                    <Link href="/profile" className="avatar-menu__link" onClick={() => setUserMenuOpen(false)}>
                      Profile
                    </Link>
                    {canAccessCreatorTools && (
                      <Link href={studioHref} className="avatar-menu__link" onClick={() => setUserMenuOpen(false)}>
                        Creator studio
                      </Link>
                    )}
                    {isAdmin && (
                      <a href={adminUrl} className="avatar-menu__link" onClick={() => setUserMenuOpen(false)}>
                        Control center
                      </a>
                    )}
                    <button
                      type="button"
                      className="avatar-menu__link"
                      onClick={() => {
                        setUserMenuOpen(false);
                        void signOut();
                      }}
                    >
                      Sign out
                    </button>
                  </div>
                </div>
              ) : (
                <div className="auth-buttons">
                  <button type="button" className={signupUrl ? "ghost-button" : "accent-button"} onClick={handleSignIn}>
                    Sign in
                  </button>
                  {signupUrl && (
                    <button type="button" className="accent-button" onClick={handleJoin}>
                      Join
                    </button>
                  )}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>

      {menuOpen && isMobileDrawerPresentation && (
        <button
          type="button"
          className="nav-drawer-backdrop"
          aria-label="Close navigation menu"
          onClick={closeMenu}
        />
      )}

      <div
        id="viewer-nav-menu"
        ref={navDrawerRef}
        className={`nav-drawer${menuOpen ? " nav-drawer--open" : ""}`}
        hidden={!menuOpen}
        aria-hidden={!menuOpen}
        role={menuOpen && isMobileDrawerPresentation ? "dialog" : undefined}
        aria-modal={menuOpen && isMobileDrawerPresentation ? "true" : undefined}
      >
        <div className="nav-drawer__header">
          <div className="stack stack--2xs">
            <span className="navbar-context__eyebrow">Navigation</span>
            <h2>Browse BitRiver Live</h2>
          </div>
          <button type="button" className="secondary-button" onClick={closeMenu}>
            Close
          </button>
        </div>

        <nav className="nav-drawer__section" aria-label="Primary navigation mobile">
          {navItems.map((item) => {
            const active = isRouteActive(item.href);
            return (
              <Link
                key={item.href}
                href={item.href}
                className={`nav-drawer__link${active ? " nav-drawer__link--active" : ""}`}
                aria-current={active ? "page" : undefined}
                onClick={closeMenu}
              >
                {item.label}
              </Link>
            );
          })}
        </nav>

        <form className="nav-search nav-search--drawer" role="search" onSubmit={handleSearch}>
          <label className="sr-only" htmlFor="navbar-search-mobile">
            Search for channels or categories
          </label>
          <input
            id="navbar-search-mobile"
            className="nav-search__input"
            type="search"
            placeholder="Search channels, creators, or tags"
            value={searchQuery}
            onChange={(event) => setSearchQuery(event.target.value)}
          />
          <button type="submit" className="nav-search__submit">
            Search
          </button>
        </form>

        <div className="nav-drawer__section" aria-label="Platform shortcuts">
          {quickLinks.map((item) => (
            <Link key={item.href} href={item.href} className="nav-drawer__link" onClick={closeMenu}>
              {item.label}
            </Link>
          ))}
          {user && (
            <Link href="/profile" className="nav-drawer__link" onClick={closeMenu}>
              Profile
            </Link>
          )}
          {canAccessCreatorTools && (
            <Link href={studioHref} className="nav-drawer__link" onClick={closeMenu}>
              Creator studio
            </Link>
          )}
          {isAdmin && (
            <a href={adminUrl} className="nav-drawer__link" onClick={closeMenu}>
              Control center
            </a>
          )}
        </div>

        {user ? (
          <div className="nav-drawer__account surface">
            <div className="stack stack--2xs">
              <span className="navbar-context__eyebrow">Account</span>
              <strong>{user.displayName}</strong>
            </div>
            <button
              type="button"
              className="secondary-button"
              onClick={() => {
                closeMenu();
                void signOut();
              }}
            >
              Sign out
            </button>
          </div>
        ) : (
          <div className="nav-drawer__cta">
            <button type="button" className={signupUrl ? "ghost-button" : "accent-button"} onClick={handleSignIn}>
              Sign in
            </button>
            {signupUrl && (
              <button type="button" className="accent-button" onClick={handleJoin}>
                Join
              </button>
            )}
          </div>
        )}
      </div>
    </header>
  );
}
