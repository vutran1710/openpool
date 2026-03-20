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
  - Controls schema.yaml, matcher.yaml, pool.json
  - Runs the relay (has POOL_SALT only — cannot decrypt profiles)
  - Gets index.db (vectors + public attributes, no raw profiles)
  - Never touches OPERATOR_PRIVATE_KEY (doesn't have it)
```

### What Pool Operators Control
- Matching rules (matcher.yaml)
- Schema definition (schema.yaml)
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

## Current Mitigations (Before Any Redesign)

1. **Private attributes** — peer-to-peer encrypted, operator never sees them
2. **CLI workflow validation** — scan all Actions in pool repo, warn on unrecognized workflows
3. **Registry vetting** — registry maintainer reviews pools before listing
4. **Operator-signed comments** — prevents comment injection even if pool repo is public
5. **Key rotation** — operator can rotate key, users re-register (documented recovery path)
6. **Minimize public attributes** — schema should encourage marking sensitive fields as private
