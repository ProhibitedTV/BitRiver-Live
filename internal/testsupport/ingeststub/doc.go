// Package ingeststub hosts deterministic HTTP control-plane fakes for ingest
// integration tests. The helpers here simulate SRS channel management, OME
// application lifecycle, and transcoder job control without touching the
// network, enabling end-to-end ingest orchestration tests to assert control
// calls and retries.
package ingeststub
