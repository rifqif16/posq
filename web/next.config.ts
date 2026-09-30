import type { NextConfig } from "next";

// Browser hanya bicara ke origin web; /api/* diteruskan ke Go API.
// Dengan begitu cookie refresh (SameSite=Strict) tetap same-origin dan tanpa CORS.
const API_URL = process.env.API_URL ?? "http://localhost:8080";

const nextConfig: NextConfig = {
  async rewrites() {
    return [{ source: "/api/:path*", destination: `${API_URL}/:path*` }];
  },
};

export default nextConfig;
