import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Produces a minimal self-contained server bundle in .next/standalone,
  // which the Dockerfile copies straight into the runtime image.
  output: "standalone",
};

export default nextConfig;
