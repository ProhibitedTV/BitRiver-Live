import type { Metadata } from "next";
import { ReactNode } from "react";
import "../styles/globals.css";
import "../styles/home.css";
import { Providers } from "../components/Providers";
import { Navbar } from "../components/Navbar";

export const metadata: Metadata = {
  title: {
    default: "BitRiver Live | Community Broadcasts",
    template: "%s | BitRiver Live"
  },
  description:
    "Discover community-powered broadcasts on BitRiver Live. Browse live channels, follow creators, and watch low-latency streams from your self-hosted portal.",
  metadataBase: new URL("https://viewer.localhost"),
  openGraph: {
    title: "BitRiver Live | Community Broadcasts",
    description:
      "Discover community-powered broadcasts on BitRiver Live. Browse live channels, follow creators, and watch low-latency streams from your self-hosted portal.",
    type: "website"
  },
  twitter: {
    card: "summary_large_image",
    title: "BitRiver Live",
    description:
      "Discover community-powered broadcasts on BitRiver Live. Browse live channels, follow creators, and watch low-latency streams from your self-hosted portal."
  }
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body>
        <Providers>
          <Navbar />
          <main>{children}</main>
          <footer className="footer">
            Crafted for self-hosted creators · Powered by BitRiver Live
          </footer>
        </Providers>
      </body>
    </html>
  );
}
