# Trust Model — Registry vs Pool Operator

## Status: Brainstorming

## The Problem

The system is not fully decentralized. Users encrypt their profiles to the pool operator's pubkey. If the operator is compromised (key leaked, malicious actor), all profile data is exposed. Additionally, pool operators can add arbitrary GitHub Actions workflows that access the operator private key.

## Current Trust Model

```
User → encrypts profile to POOL OPERATOR's pubkey
Pool operator:
  - Decrypts profiles (has OPERATOR_PRIVATE_KEY)
  - Runs indexer (builds index.db)
  - Runs registration Action (computes hashes)
  - Runs interest Action (detects matches)
  - Controls the relay server (has POOL_SALT)
  - Can add arbitrary workflows that access secrets
```

**Users trust:** the pool operator with their public attributes + the security of the operator's private key.

**Private attributes** (visibility: private) are peer-to-peer encrypted and never seen by the operator — only revealed after mutual match via ECDH.

## Proposed: Registry as Single Trust Root

Move the master key from pool operators to the registry. Pool operators never see raw profile data.

```
User → encrypts profile to REGISTRY's pubkey (not pool operator)

Registry:
  - Holds the master decryption key
  - Runs the indexer (centralized service or Action)
  - Decrypts .bin → validates → builds index.db (public attrs only)
  - Commits index.db to pool repo

Pool operator:
  - Controls pool.yaml
  - Runs the relay (has POOL_SALT only — cannot decrypt profiles)
  - Gets index.db (vectors + public attributes, no raw profiles)
  - Never touches OPERATOR_PRIVATE_KEY (doesn't have it)
```

### What Pool Operators Control
- Matching rules (pool.yaml matching rules)
- Schema definition (pool.yaml schema)
- Relay server (routing only, zero knowledge)
- Pool metadata

### What Pool Operators Can't Do
- Decrypt user profiles
- Access private attributes
- Run arbitrary code that touches encrypted data

### What the Registry Controls
- Master decryption key
- Indexer service
- Pool vetting (which pools are listed)
- Profile decryption + indexing

## Trade-offs

| | Current (operator-trusted) | Proposed (registry-trusted) |
|---|---|---|
| Trust boundary | Per-pool operator | Single registry |
| Compromise blast radius | One pool | All pools in registry |
| Operator flexibility | Full control | Schema + matcher only |
| Decentralization | More distributed (each pool independent) | More centralized (registry is bottleneck) |
| Key management | Per-pool keys | One registry key |
| Recovery from compromise | Re-register in one pool | Re-register everything |

## Open Questions

- Can the registry be federated (multiple registries, each with own key)?
- Should the registry indexer be a hosted service or a GitHub Action on the registry repo?
- How does the pool operator run registration Actions if they don't have the key?
  - Option A: Registry runs all Actions (pool repo is just data storage)
  - Option B: Pool repo Actions call a registry API to decrypt/sign
  - Option C: Registry generates per-pool ephemeral keys (scoped, revocable)
- Is Shamir's Secret Sharing relevant here? (Split registry key, require N-of-M to decrypt)
- How does this affect the relay? (Relay needs POOL_SALT but not OPERATOR_PRIVATE_KEY — already separated)

## Hash Chain — Honest Assessment

The hash chain (`id_hash → bin_hash → match_hash`) provides **relay-level privacy** — the relay operator sees `match_hash` but can't link it to a GitHub user without the salt. But it does **not** protect against a determined observer watching the public pool repo.

**Computational linking** (blocked by salt): without the salt, an observer can't derive `bin_hash → match_hash`. The chain is cryptographically sound.

**Observational linking** (timing side-channel): when a new `.bin` file is committed and a new entry appears in `index.db`, the observer can correlate them by timing. In a small pool (50 users), new registrations are obvious.

**Mitigation: History squashing.** A periodic cron Action squashes the pool repo to a single commit. All `.bin` files + `index.db` appear simultaneously — no timeline to correlate.

```
Before squash:
  commit abc: Register user a1b2.bin     ← Mar 15
  commit def: Register user d4e5.bin     ← Mar 16
  commit ghi: Rebuild index.db          ← Mar 16
  → attacker links d4e5 to new index entry

After squash:
  commit xyz: Pool state                ← single commit
  → all files appear at once, no correlation
```

Gets stronger over time as pool grows (more users = more noise per squash). Cheap to implement — just a cron Action that force-pushes a squashed commit.

## Private Key — The Real Trust Boundary

The user's ed25519 private key is the ultimate point of failure. If stolen, everything is compromised for that user:

1. Match pubkey against `.bin` files → find bin_hash
2. Decrypt registration comment → get match_hash
3. Decrypt match notifications → get peer pubkeys
4. Authenticate to relay → full impersonation

No amount of hash chain complexity, link secrets, or additional hashing helps — everything is ultimately encrypted to the user's pubkey and delivered via issue/PR comments.

**This is acceptable** — same threat model as SSH, PGP, Signal. Mitigations:
- Key file has `0600` permissions
- Recovery: generate new keypair, re-register
- Blast radius: one user only, no impact on other users or the pool

## Current Mitigations (Before Any Redesign)

1. **Private attributes** — peer-to-peer encrypted, operator never sees them
2. **CLI workflow validation** — scan all Actions in pool repo, warn on unrecognized workflows
3. **Registry vetting** — registry maintainer reviews pools before listing
4. **Operator-signed comments** — prevents comment injection even if pool repo is public
5. **Key rotation** — operator can rotate key, users re-register (documented recovery path)
6. **Minimize public attributes** — schema should encourage marking sensitive fields as private
