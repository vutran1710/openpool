# Relay Auth Security Analysis

## Status: Implemented — channel binding (PR #39), full analysis in SECURITY_DESIGN.md

## Mechanism

Time-based public key authentication — closest to SSH key auth with TOTP time windowing.

```
Client: sign(sha256(time_window)) with ed25519 private key
Relay:  validate id_hash → bin_hash → match_hash chain (has salt)
        fetch pubkey from GitHub raw content
        verify signature with pubkey
        ±1 window (15 min total tolerance)
```

## Strengths

- No shared secrets (asymmetric ed25519)
- No tokens, no sessions, no JWTs — stateless
- Replay window limited to ~15 min
- Identity binding via chain validation (can't claim someone else's match_hash)
- Private key never leaves client

## Known Concerns

| Issue | Severity | Current Mitigation |
|-------|----------|-------------------|
| Signature reuse within window | Low | TLS protects upgrade URL from interception |
| No channel binding | Medium | TLS verifies relay server identity |
| Pubkey trust (GitHub as source of truth) | Medium | Only Actions can write .bin files (repo permissions) |
| No mutual auth | Low | Standard for WebSocket services |

## Improvement: Channel Binding

Add relay hostname to signed message to prevent cross-relay signature reuse:

```
Current:  sign(sha256(time_window))
Proposed: sign(sha256(time_window + relay_host))
```

This ensures a signature for `relay-a.example.com` can't be accepted by `relay-b.example.com`. Defense in depth — TLS already handles server identity, but this binds the signature to the intended destination.

Requires:
- Client knows the relay hostname (already has it from pool config)
- Relay knows its own hostname (from env var or request Host header)
- TOTP functions updated: `TOTPSign(priv, relayHost)` / `TOTPVerify(sig, pub, relayHost)`
