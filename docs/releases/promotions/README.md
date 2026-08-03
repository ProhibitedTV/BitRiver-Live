# Stable promotion records

Commit one reviewed `<stable-tag>.json` record here only after every required
gate has durable evidence for the same signed candidate release-set SHA-256.
Start from [`../promotion-record.example.json`](../promotion-record.example.json).

Example/template files are not approvals. Never copy placeholder hashes or
`example.invalid` URLs into a promotion record. The protected manual workflow
also checks the live issue states and rejects records for another candidate.
