import type { Metadata } from "next";
import { ReactNode } from "react";
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

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body>
        <Providers>
          <Navbar />
          <ViewerShell>{children}</ViewerShell>
        </Providers>
      </body>
    </html>
  );
}
