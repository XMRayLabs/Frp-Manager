const { PHASE_DEVELOPMENT_SERVER } = require('next/constants')

/** @type {import('next').NextConfig} */
const baseConfig = {
  output: 'export',
  images: {
    unoptimized: true,
  },
  reactStrictMode: true,
}

module.exports = (phase) => ({
  ...baseConfig,
  ...(phase === PHASE_DEVELOPMENT_SERVER
    ? {
        async rewrites() {
          return [{ source: '/api/:path*', destination: 'http://localhost:9000/api/:path*' }]
        },
      }
    : {}),
})
