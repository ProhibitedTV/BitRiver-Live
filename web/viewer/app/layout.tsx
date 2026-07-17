import type { Metadata } from "next";
import Link from "next/link";
import { ReactNode, Suspense } from "react";
import "../styles/globals.css";
import "../styles/navigation.css";
import "../styles/viewer-shell.css";
import "../styles/directory.css";
import "../styles/channel-watch.css";
import "../styles/chat.css";
import "../styles/creator-studio.css";
import "../styles/shared-polish.css";
import "../styles/responsive.css";
import "../styles/home.css";
import "../styles/network-console.css";
import { Providers } from "../components/Providers";
import { Navbar } from "../components/Navbar";
import { ViewerShell } from "../components/ViewerShell";

export const metadata: Metadata = {
  title: {
    default: "BitRiver Live | Live channels and creator tools",
    template: "%s | BitRiver Live"
  },
  description:
    "Watch live channels, follow creators, tip directly, catch replays, and launch your own stream with BitRiver Live.",
  metadataBase: new URL("https://viewer.localhost"),
  openGraph: {
    title: "BitRiver Live | Live channels and creator tools",
    description:
      "Watch live channels, follow creators, tip directly, catch replays, and launch your own stream with BitRiver Live.",
    type: "website"
  },
  twitter: {
    card: "summary_large_image",
    title: "BitRiver Live",
    description:
      "Watch live channels, follow creators, tip directly, catch replays, and launch your own stream with BitRiver Live."
  }
};

function NavbarFallback() {
  return (
    <header className="navbar" aria-label="Loading primary navigation">
      <div className="navbar-inner">
        <div className="navbar-main">
          <div className="navbar-branding">
            <Link href="/" aria-label="BitRiver Live home" className="navbar-logo">
              <span className="navbar-logo__icon" aria-hidden="true">BR</span>
              <span className="navbar-logo__copy">
                <span className="navbar-logo__text">BitRiver Live</span>
              </span>
            </Link>
          </div>
        </div>
      </div>
    </header>
  );
}

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body>
        <Providers>
          <Suspense fallback={<NavbarFallback />}>
            <Navbar />
          </Suspense>
          <ViewerShell>{children}</ViewerShell>
        </Providers>
      </body>
    </html>
  );
}
