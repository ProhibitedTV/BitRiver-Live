// Package server hosts the BitRiver API and control centre from a single HTTP server.
//
// The server builds a consistent middleware chain of auth, rate limiting,
// metrics, audit, and logging so handlers all share common protections and
// instrumentation.
//
// It serves API routes, embeds the static control centre assets, and proxies the
// viewer when configured, keeping everything behind one multiplexer.
package server
