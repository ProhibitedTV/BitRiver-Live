"use client";

import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { FormEvent, useEffect, useMemo, useState } from "react";
import { useAuth } from "../hooks/useAuth";
import { deriveQuickLinks, getNavigationAudience, getVisibleNavigationItems } from "../lib/navigation";
import { fetchManagedChannels } from "../lib/viewer-api";

const LOCAL_SETUP_DOCS_ROUTE = "/getting-started";

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
  const searchParams = useSearchParams();
  const searchParamQuery = searchParams?.get("q") ?? "";
  const isAdmin = Boolean(user?.roles?.includes("admin"));
  const isCreator = Boolean(user?.roles?.includes("creator"));
  const canAccessCreatorTools = isAdmin || isCreator;
  const [theme, setTheme] = useState<"dark" | "light">("dark");
  const [managedChannelId, setManagedChannelId] = useState<string | undefined>();
  const [searchQuery, setSearchQuery] = useState(searchParamQuery);
  const [menuOpen, setMenuOpen] = useState(false);
  const [userMenuOpen, setUserMenuOpen] = useState(false);
  const pathname = usePathname();
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
  const configuredSignupUrl = process.env.NEXT_PUBLIC_SIGNUP_URL?.trim();
  const shouldShowLocalSetupBanner =
    process.env.NODE_ENV !== "production" &&
    (isLocalhostUrl(process.env.NEXT_PUBLIC_VIEWER_URL) || isLocalhostUrl(process.env.NEXT_PUBLIC_API_BASE_URL));
  const signupUrl = useMemo(() => {
    if (configuredSignupUrl !== undefined) {
      return configuredSignupUrl || undefined;
    }
    if (process.env.NEXT_PUBLIC_API_BASE_URL) {
      return `${process.env.NEXT_PUBLIC_API_BASE_URL}/signup`;
    }
    return "/signup";
  }, [configuredSignupUrl]);
  const quickLinks = useMemo(() => deriveQuickLinks(navigationAudience, navItems), [navigationAudience, navItems]);
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
    if (!window.matchMedia) {
      return;
    }
    const query = window.matchMedia("(prefers-color-scheme: light)");
    const setFromQuery = (matches: boolean) => setTheme(matches ? "light" : "dark");
    setFromQuery(query.matches);
    const handler = (event: MediaQueryListEvent) => setFromQuery(event.matches);
    query.addEventListener("change", handler);
    return () => {
      query.removeEventListener("change", handler);
    };
  }, []);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    if (!window.matchMedia) {
      return;
    }
    const query = window.matchMedia("(min-width: 640px)");
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
    // Contract: treat env-configured signup targets as either absolute or relative URLs,
    // and normalize them against the current origin before mutating query params.
    const url = new URL(signupUrl, window.location.origin);
    // Contract: preserve an explicitly configured `next`; only synthesize one from the
    // current pathname/search/hash when config does not provide it.
    if (!url.searchParams.has("next")) {
      url.searchParams.set("next", buildRedirectTarget());
    }
    window.location.href = url.toString();
  };

  const handleSearch = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const trimmed = searchQuery.trim();
    // Contract: empty search maps to `/browse`; non-empty searches remain encoded in `q`.
    await router.push(trimmed ? `/browse?q=${encodeURIComponent(trimmed)}` : "/browse");
    // Contract: close menu only after issuing navigation so mobile drawer state follows route intent.
    closeMenu();
  };

  const avatarGlyph = useMemo(() => {
    if (!user?.displayName) {
      return "👤";
    }
    return user.displayName.trim().charAt(0).toUpperCase();
  }, [user?.displayName]);

  return (
    <header className="navbar">
      {shouldShowLocalSetupBanner && (
        <div className="local-setup-banner" role="status">
          <span>Running in local setup mode. Before going public, configure your domain + CORS settings.</span>{" "}
          <Link href={LOCAL_SETUP_DOCS_ROUTE} className="local-setup-banner__link" onClick={closeMenu}>
            Setup guide
          </Link>
        </div>
      )}
      <div className="navbar-inner">
        {/* navbar-left contract: anchor brand + primary destinations; desktop keeps this always visible while mobile mirrors links in drawer. */}
        <div className="navbar-left" aria-hidden={menuOpen}>
          <Link href="/" aria-label="BitRiver Live home" className="navbar-logo" onClick={closeMenu}>
            <span className="navbar-logo__icon" aria-hidden>
              📡
            </span>
            <span className="navbar-logo__text">BitRiver Live</span>
          </Link>
          <nav className="nav-tabs" role="group" aria-label="Viewer navigation">
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
        </div>
        <button
          className="nav-toggle"
          type="button"
          aria-expanded={menuOpen}
          aria-controls="viewer-nav-menu"
          aria-label={menuOpen ? "Close navigation menu" : "Open navigation menu"}
          onClick={() => setMenuOpen((prev) => !prev)}
        >
          <span aria-hidden>{menuOpen ? "✕" : "☰"}</span>
        </button>
        {/* navbar-center contract: shared search state/submit behavior between inline and drawer forms for responsive parity. */}
        <div className="navbar-center">
          <form className="nav-search nav-search--inline" role="search" onSubmit={handleSearch}>
            <label className="sr-only" htmlFor="navbar-search">
              Search for channels or categories
            </label>
            <input
              id="navbar-search"
              className="nav-search__input"
              type="search"
              placeholder="Search"
              value={searchQuery}
              onChange={(event) => setSearchQuery(event.target.value)}
            />
            <button type="submit" className="icon-button" aria-label="Search">
              🔍
            </button>
          </form>
        </div>
        {/* navbar-right contract: render role/user-aware actions (creator CTA, theme, auth/account) without diverging desktop/mobile behavior. */}
        <div className="navbar-right">
          {canAccessCreatorTools && managedChannelId && (
            <Link href={`/creator/live/${managedChannelId}`} className="nav-cta" onClick={closeMenu}>
              Go live
            </Link>
          )}
          <div className="nav-icon-group" role="group" aria-label="Viewer quick actions">
            <button
              className="icon-button"
              type="button"
              aria-label="View notifications"
              disabled
              title="Notifications coming soon"
            >
              🔔
            </button>
            <button
              className="icon-button"
              type="button"
              onClick={() => setTheme((prev) => (prev === "light" ? "dark" : "light"))}
              aria-label={`Switch to ${theme === "light" ? "dark" : "light"} theme`}
            >
              {theme === "light" ? "🌙" : "🌞"}
            </button>
            {user ? (
              <div className="avatar-menu" aria-label="Account menu">
                <button
                  type="button"
                  className="avatar-button"
                  aria-label="Open account menu"
                  aria-expanded={userMenuOpen}
                  onClick={() => setUserMenuOpen((prev) => !prev)}
                >
                  {avatarGlyph}
                </button>
                <div className={`avatar-menu__items${userMenuOpen ? " avatar-menu__items--open" : ""}`}>
                  <div className="avatar-menu__header">
                    <span className="muted">Signed in as</span>
                    <span className="avatar-menu__name">{user.displayName}</span>
                  </div>
                  <Link href="/profile" className="avatar-menu__link" onClick={() => setUserMenuOpen(false)}>
                    Profile
                  </Link>
                  {canAccessCreatorTools && (
                    <Link
                      href={managedChannelId ? `/creator/live/${managedChannelId}` : "/creator"}
                      className="avatar-menu__link"
                      onClick={() => setUserMenuOpen(false)}
                    >
                      Creator tools
                    </Link>
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
      <div
        id="viewer-nav-menu"
        className={`nav-drawer${menuOpen ? " nav-drawer--open" : ""}`}
        hidden={!menuOpen}
        aria-hidden={!menuOpen}
      >
        <div className="nav-drawer__section" role="group" aria-label="Viewer navigation mobile">
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
        </div>
        <form className="nav-search nav-search--drawer" role="search" onSubmit={handleSearch}>
          <label className="sr-only" htmlFor="navbar-search-mobile">
            Search for channels or categories
          </label>
          <input
            id="navbar-search-mobile"
            className="nav-search__input"
            type="search"
            placeholder="Search"
            value={searchQuery}
            onChange={(event) => setSearchQuery(event.target.value)}
          />
          <button type="submit" className="icon-button" aria-label="Search">
            🔍
          </button>
        </form>
        <div className="nav-drawer__section" role="group" aria-label="Quick links">
          {quickLinks.map((item) => (
            <Link key={item.href} href={item.href} className="nav-drawer__link" onClick={closeMenu}>
              {item.label}
            </Link>
          ))}
          {canAccessCreatorTools && managedChannelId && (
            <Link
              href={`/creator/live/${managedChannelId}`}
              className="nav-drawer__link"
              onClick={closeMenu}
            >
              Creator tools
            </Link>
          )}
          {!user && (
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
      </div>
    </header>
  );
}
