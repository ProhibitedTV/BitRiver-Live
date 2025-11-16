"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { FormEvent, useEffect, useState } from "react";
import { useAuth } from "../hooks/useAuth";
import { fetchManagedChannels } from "../lib/viewer-api";

export function Navbar() {
  const { user, login, signup, logout, error } = useAuth();
  const router = useRouter();
  const isAdmin = Boolean(user?.roles?.includes("admin"));
  const isCreator = Boolean(user?.roles?.includes("creator"));
  const canAccessCreatorTools = isAdmin || isCreator;
  const [mode, setMode] = useState<"login" | "signup" | null>(null);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [message, setMessage] = useState<string | undefined>();
  const [theme, setTheme] = useState<"dark" | "light">("dark");
  const [managedChannelId, setManagedChannelId] = useState<string | undefined>();
  const [searchQuery, setSearchQuery] = useState("");
  const [menuOpen, setMenuOpen] = useState(false);
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
    ...(canAccessCreatorTools ? [{ label: "Creator", href: "/creator" }] : []),
  ];
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
  }, [user]);

  useEffect(() => {
    setMenuOpen(false);
  }, [mode, user]);

  const reset = () => {
    setEmail("");
    setPassword("");
    setDisplayName("");
    setMessage(undefined);
  };

  const closeMenu = () => {
    setMenuOpen(false);
  };

  const handleSearch = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const trimmed = searchQuery.trim();
    await router.push(trimmed ? `/browse?q=${encodeURIComponent(trimmed)}` : "/browse");
    closeMenu();
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
        <div className="navbar-brand">
          <Link href="/" aria-label="BitRiver Live home" className="badge" onClick={closeMenu}>
            <span role="img" aria-hidden>
              📡
            </span>
            BitRiver Live
          </Link>
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
        </div>
        <nav id="viewer-nav-menu" className={`nav-links${menuOpen ? " nav-links--open" : ""}`}>
          <div className="nav-tabs" role="group" aria-label="Viewer navigation">
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
          </div>
          <div className="nav-actions">
            <form className="nav-search" role="search" onSubmit={handleSearch}>
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
              <button type="submit" className="secondary-button nav-search__button">
                Go
              </button>
            </form>
            <div className="nav-quick-links" role="group" aria-label="Quick links">
              <Link href="/browse" className="nav-pill" onClick={closeMenu}>
                Categories
              </Link>
              <Link href="/following" className="nav-pill" onClick={closeMenu}>
                Following
              </Link>
            </div>
          </div>
          <button
            className="secondary-button"
            type="button"
            onClick={() => setTheme((prev) => (prev === "light" ? "dark" : "light"))}
            aria-label={`Switch to ${theme === "light" ? "dark" : "light"} theme`}
          >
            {theme === "light" ? "🌙 Dark" : "🌞 Light"}
          </button>
          {user ? (
            <>
              <span className="muted">Signed in as {user.displayName}</span>
              {isAdmin && (
                <Link href="/" className="secondary-button" onClick={closeMenu}>
                  Dashboard
                </Link>
              )}
              {managedChannelId && (
                <Link
                  href={`/creator/uploads/${managedChannelId}`}
                  className="secondary-button"
                  onClick={closeMenu}
                >
                  Manage channel
                </Link>
              )}
              <button
                className="secondary-button"
                onClick={() => {
                  closeMenu();
                  void logout();
                }}
              >
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
                  closeMenu();
                }}
              >
                Sign in
              </button>
              <button
                className="primary-button"
                onClick={() => {
                  reset();
                  setMode("signup");
                  closeMenu();
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
