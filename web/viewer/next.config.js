const basePath = process.env.NEXT_VIEWER_BASE_PATH?.trim();

/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  images: {
    unoptimized: true
  },
  output: "standalone",
  basePath: basePath ? basePath : undefined
};

module.exports = nextConfig;
