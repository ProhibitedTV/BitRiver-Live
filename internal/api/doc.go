// Package api hosts HTTP handlers that front the BitRiver REST API.
//
// The handlers assembled by Handler coordinate request validation, session
// awareness, and response shaping while delegating persistence to
// storage.Repository implementations injected at construction time.
// Authentication and session lifecycle management are provided by
// auth.SessionManager instances passed into the handler; the package does not
// reach for globals or singletons and expects callers to supply fully
// configured dependencies.
//
// Queue clients and health probes are also injected so background upload,
// transcoding, and availability checks can be triggered without coupling the
// package to specific runtime wiring. This keeps endpoint behaviour testable
// and aligned with the wider service architecture.
//
// Handler implementations assume upstream middleware from internal/server has
// already enforced authentication, rate limiting, metrics, auditing, and
// logging concerns. New routes should preserve that contract by avoiding
// duplicate validation and by leaning on the middleware guarantees established
// in the server stack.
package api
