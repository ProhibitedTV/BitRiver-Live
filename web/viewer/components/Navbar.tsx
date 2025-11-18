"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { FormEvent, useEffect, useMemo, useState } from "react";
import { useAuth } from "../hooks/useAuth";
import { fetchManagedChannels } from "../lib/viewer-api";
import { AuthDialog } from "./auth/AuthDialog";

export function Navbar() {
  const { user, login, signup, logout, error } = useAuth();
  const router = useRouter();
  const isAdmin = Boolean(user?.roles?.includes("admin"));
  const isCreator = Boolean(user?.roles?.includes("creator"));
  const canAccessCreatorTools = isAdmin || isCreator;
  const [mode, setMode] = useState<"login" | "signup" | null>(null);
  const [theme, setTheme] = useState<"dark" | "light">("dark");
  const [managedChannelId, setManagedChannelId] = useState<string | undefined>();
  const [searchQuery, setSearchQuery] = useState("");
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
  const navItems = [
    { label: "Home", href: "/" },
    { label: "Following", href: "/following" },
    { label: "Browse", href: "/browse" },
    ...(isAdmin ? [{ label: "Dashboard", href: "/dashboard" }] : []),
    ...(canAccessCreatorTools ? [{ label: "Creator", href: "/creator" }] : []),
  ];
  const navItemHrefs = useMemo(() => new Set(navItems.map((item) => item.href)), [navItems]);
  const quickLinks = [
    { label: "Categories", href: "/browse" },
    { label: "Following", href: "/following" },
  ].filter((item) => !navItemHrefs.has(item.href));
  const isRouteActive = (href: string) => {
    if (href === "/") {
      return canonicalPath === "/";
    }
    return canonicalPath === href || canonicalPath.startsWith(`${href}/`);
  };

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
  }, [mode, user]);

  const closeMenu = () => {
    setMenuOpen(false);
  };

  const handleSearch = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const trimmed = searchQuery.trim();
    await router.push(trimmed ? `/browse?q=${encodeURIComponent(trimmed)}` : "/browse");
    closeMenu();
  };

  const handleAuthClose = () => setMode(null);

  const handleLogin = async (email: string, password: string) => login(email, password);

  const handleSignup = async (displayName: string, email: string, password: string) =>
    signup(displayName, email, password);

  const avatarGlyph = useMemo(() => {
    if (!user?.displayName) {
      return "👤";
    }
    return user.displayName.trim().charAt(0).toUpperCase();
  }, [user?.displayName]);

  return (
    <header className="navbar">
      <div className="navbar-inner">
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
        <div className="navbar-right">
          {canAccessCreatorTools && managedChannelId && (
            <Link href={`/creator/live/${managedChannelId}`} className="nav-cta" onClick={closeMenu}>
              Go live
            </Link>
          )}
          <div className="nav-icon-group" role="group" aria-label="Viewer quick actions">
            <button className="icon-button" type="button" aria-label="View notifications">
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
                      void logout();
                    }}
                  >
                    Sign out
                  </button>
                </div>
              </div>
            ) : (
              <div className="auth-buttons">
                <button
                  className="ghost-button"
                  onClick={() => {
                    setMode("login");
                    closeMenu();
                  }}
                >
                  Sign in
                </button>
                <button
                  className="accent-button"
                  onClick={() => {
                    setMode("signup");
                    closeMenu();
                  }}
                >
                  Sign up
                </button>
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
              <button
                className="ghost-button"
                onClick={() => {
                  setMode("login");
                  closeMenu();
                }}
              >
                Sign in
              </button>
              <button
                className="accent-button"
                onClick={() => {
                  setMode("signup");
                  closeMenu();
                }}
              >
                Sign up
              </button>
            </div>
          )}
        </div>
      </div>
      <AuthDialog
        mode={mode}
        onClose={handleAuthClose}
        onLogin={handleLogin}
        onSignup={handleSignup}
        authError={error}
      />
    </header>
  );
}
