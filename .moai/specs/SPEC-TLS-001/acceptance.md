---
id: SPEC-TLS-001
type: acceptance
updated: "2026-06-03"
---

# SPEC-TLS-001 Acceptance Criteria

## AC1: Provider disabled (no secret ID)

- GIVEN `SMTP_PROXY_TLS_SECRET_ID` is empty
- WHEN the SMTP server starts and a client performs STARTTLS
- THEN the server serves a self-signed certificate
- AND behavior is identical to the pre-change implementation

## AC2: Single-domain secret loaded at startup

- GIVEN a secret containing one domain with a valid cert/key
- WHEN the server starts with `SMTP_PROXY_TLS_SECRET_ID` set
- THEN that certificate is served for handshakes with and without SNI

## AC3: Multi-domain SNI selection

- GIVEN a secret containing `a.example.com` and `b.example.com`
- WHEN a handshake includes SNI `b.example.com`
- THEN the `b.example.com` certificate is served

## AC4: SNI miss falls back to self-signed

- GIVEN a secret containing only `a.example.com`
- WHEN a handshake includes SNI `unknown.example.com`
- THEN a self-signed certificate is served for that handshake
- AND other handshakes are unaffected

## AC5: No-SNI default selection

- GIVEN a multi-domain secret and `SMTP_PROXY_TLS_DEFAULT_CERT=b.example.com`
- WHEN a handshake carries no SNI
- THEN the `b.example.com` certificate is served
- AND WHEN `SMTP_PROXY_TLS_DEFAULT_CERT` is unset, the first domain in sorted
  order is served deterministically

## AC6: Startup fetch failure falls back to self-signed

- GIVEN `SMTP_PROXY_TLS_SECRET_ID` points to a missing/unreadable secret
- WHEN the server starts
- THEN it logs a warning and serves self-signed without crashing

## AC7: Reload picks up a new certificate

- GIVEN the server started with a valid secret
- WHEN the secret value changes and the reload interval elapses
- THEN the new certificate is served without restart

## AC8: Reload failure retains last-good certificate

- GIVEN the server is serving a Secrets Manager certificate
- WHEN a subsequent reload fails (fetch or parse error)
- THEN the previous certificate continues to be served
- AND a warning is logged
- AND the next interval retries

## AC9: Self-signed start promotes to real cert on later success

- GIVEN the server started self-signed because the secret was empty/invalid
- WHEN the secret later becomes valid and the interval elapses
- THEN the server promotes to the Secrets Manager certificate

## AC10: Partial failure isolation

- GIVEN a multi-domain secret where one entry has an invalid key
- WHEN the secret is loaded
- THEN the invalid entry is skipped and valid entries are served

## AC11: Default reload interval

- GIVEN `SMTP_PROXY_TLS_RELOAD_INTERVAL` is unset
- WHEN configuration is loaded
- THEN the interval defaults to 168 hours

## AC12: Concurrency safety

- GIVEN concurrent handshakes during a reload
- WHEN `go test -race` runs the provider tests
- THEN no data race is reported

## Quality Gates

- `go test -race ./...` passes
- `go vet ./...` clean
- New package coverage ≥ 85%
- `tls.mode = none` path unchanged
