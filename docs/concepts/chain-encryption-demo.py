"""
Chain Encryption Demo v4 — Sequential profile discovery with hint-based acceleration.

Model:
  - Each profile is AES-256-GCM encrypted
  - Key = sha256("{a}:{b}:{c}") where a,b,c are hint, profile_constant, nonce
    in a random order (determined by chain context, unknown to explorer)

  Three components:
    hint:             8-digit number (10^8 range), revealed by previous box
    profile_constant: 4-digit number (10^4 range), derived from profile content
    nonce:            N range (pool-configurable), random per entry per permutation

  Security levels:
    Without hint: 10^8 * 10^4 * N * 6 = infeasible (days/years)
    With hint (cold): 10^4 * N * 6 = seconds (the normal unlock rate)
    Warm (known constant): N * 6 = instant (re-encounter advantage)

  Inside each decrypted box:
    - Profile data
    - This profile's constant (advantage for re-solving in other permutations)
    - Next hint (8-digit, enables solving the next box — without it, infeasible)

  Single tuning knob: nonce_space (N)
"""

import hashlib
import json
import os
import time
import random
from cryptography.hazmat.primitives.ciphers.aead import AESGCM

ORDERINGS = [
    (0, 1, 2), (0, 2, 1), (1, 0, 2),
    (1, 2, 0), (2, 0, 1), (2, 1, 0),
]


def derive_key(hint, constant, nonce, ordering):
    parts = [str(hint), str(constant), str(nonce)]
    ordered = [parts[ordering[0]], parts[ordering[1]], parts[ordering[2]]]
    return hashlib.sha256(":".join(ordered).encode()).digest()


def pick_ordering(context):
    h = int(hashlib.sha256(context.encode()).hexdigest(), 16)
    return ORDERINGS[h % 6]


def profile_constant(profile_bytes, space):
    h = int(hashlib.sha256(profile_bytes).hexdigest(), 16)
    return h % space


def profile_hint(profile_bytes, space):
    h = int(hashlib.sha256(b"hint:" + profile_bytes).hexdigest(), 16)
    return h % space


def ctx(seed, pos):
    return hashlib.sha256(seed + ":{}".format(pos).encode()).hexdigest()[:16]


# === Indexer ===

def build_chain(profiles, seed, hint_space, const_space, nonce_space):
    consts = []
    hints = []
    for p in profiles:
        pb = json.dumps(p, sort_keys=True).encode()
        consts.append(profile_constant(pb, const_space))
        hints.append(profile_hint(pb, hint_space))

    seed_hint = int(hashlib.sha256(seed).hexdigest(), 16) % hint_space
    boxes = []
    cur_hint = seed_hint

    for i, prof in enumerate(profiles):
        c = ctx(seed, i)
        ordering = pick_ordering(c)
        nonce_val = random.randint(0, nonce_space - 1)
        next_h = hints[i + 1] if i + 1 < len(profiles) else seed_hint

        payload = json.dumps({
            "profile": prof,
            "my_constant": consts[i],
            "next_hint": next_h,
        }, sort_keys=True).encode()

        key = derive_key(cur_hint, consts[i], nonce_val, ordering)
        gcm_nonce = os.urandom(12)
        ct = AESGCM(key).encrypt(gcm_nonce, payload, None)

        boxes.append({
            "tag": prof["match_hash"],
            "ct": ct,
            "gcm_nonce": gcm_nonce,
            "context": c,
        })
        cur_hint = next_h

    return {
        "seed": seed,
        "seed_hint": seed_hint,
        "hint_space": hint_space,
        "const_space": const_space,
        "nonce_space": nonce_space,
        "boxes": boxes,
    }


# === Explorer ===

def solve_cold(box, hint, const_space, nonce_space):
    """With hint, unknown constant + nonce + ordering."""
    attempts = 0
    for constant in range(const_space):
        for nonce_val in range(nonce_space):
            for ordering in ORDERINGS:
                attempts += 1
                key = derive_key(hint, constant, nonce_val, ordering)
                try:
                    pt = AESGCM(key).decrypt(box["gcm_nonce"], box["ct"], None)
                    return json.loads(pt), attempts
                except Exception:
                    continue
    return None, attempts


def solve_warm(box, hint, known_const, nonce_space):
    """Known constant, unknown nonce + ordering."""
    attempts = 0
    for nonce_val in range(nonce_space):
        for ordering in ORDERINGS:
            attempts += 1
            key = derive_key(hint, known_const, nonce_val, ordering)
            try:
                pt = AESGCM(key).decrypt(box["gcm_nonce"], box["ct"], None)
                return json.loads(pt), attempts
            except Exception:
                continue
    return None, attempts


def explore(chain, known=None):
    if known is None:
        known = {}
    cur_hint = chain["seed_hint"]
    results = []

    for box in chain["boxes"]:
        tag = box["tag"]

        if tag in known:
            t0 = time.time()
            payload, att = solve_warm(box, cur_hint, known[tag], chain["nonce_space"])
            elapsed = time.time() - t0
            if payload:
                print("  WARM:  {}... {:>8} attempts  {:.4f}s  -> {}".format(
                    tag[:12], att, elapsed, payload["profile"].get("name", "?")))
                cur_hint = payload["next_hint"]
                results.append(payload)
                continue

        t0 = time.time()
        payload, att = solve_cold(box, cur_hint, chain["const_space"], chain["nonce_space"])
        elapsed = time.time() - t0

        if payload:
            print("  COLD:  {}... {:>8} attempts  {:.3f}s  -> {}".format(
                tag[:12], att, elapsed, payload["profile"].get("name", "?")))
            known[tag] = payload["my_constant"]
            cur_hint = payload["next_hint"]
            results.append(payload)
        else:
            print("  FAIL:  {}...".format(tag[:12]))
            break

    return results, known


# === Demo ===

def main():
    profiles = [
        {"match_hash": "a1b2c3d4e5f6a1b2", "name": "Alice", "age": 27,
         "interests": ["hiking", "coding"]},
        {"match_hash": "b2c3d4e5f6a1b2c3", "name": "Bob", "age": 28,
         "interests": ["music", "travel"]},
        {"match_hash": "c3d4e5f6a1b2c3d4", "name": "Carol", "age": 26,
         "interests": ["coding", "food"]},
    ]

    # --- Real-world parameters (commented — too slow for demo) ---
    # hint_space     = 100_000_000  # 10^8 — without hint = infeasible
    # const_space    = 10_000       # 10^4 — 4-digit profile constant
    # nonce_space    = 20           # tunable knob

    # --- Demo parameters (fast enough to run) ---
    hint_space = 100_000    # reduced for demo (real: 10^8)
    const_space = 100       # reduced for demo (real: 10^4)
    nonce_space = 10        # tunable knob

    print("=== CONFIG ===")
    print("  hint_space     = {:>12,}  (without hint = infeasible)".format(hint_space))
    print("  const_space    = {:>12,}  (profile constant range)".format(const_space))
    print("  nonce_space    = {:>12,}  (tunable knob)".format(nonce_space))
    print("  orderings      = {:>12}  (3!)".format(6))
    print()
    print("  without hint:  {:>12,} attempts (infeasible)".format(hint_space * const_space * nonce_space * 6))
    print("  cold (w/hint): {:>12,} attempts (~seconds)".format(const_space * nonce_space * 6))
    print("  warm:          {:>12,} attempts (~instant)".format(nonce_space * 6))

    # Build two permutations
    perm0 = profiles[:]
    perm1 = profiles[:]
    random.seed(99)
    random.shuffle(perm1)

    print("\n=== INDEXER ===")
    print("  Perm 0: {}".format(" -> ".join(p["name"] for p in perm0)))
    print("  Perm 1: {}".format(" -> ".join(p["name"] for p in perm1)))

    chain0 = build_chain(perm0, b"perm0", hint_space, const_space, nonce_space)
    chain1 = build_chain(perm1, b"perm1", hint_space, const_space, nonce_space)

    # Perm 0: cold
    print("\n=== EXPLORER: Permutation 0 (cold — no prior knowledge) ===")
    t0 = time.time()
    r0, known = explore(chain0)
    total0 = time.time() - t0
    print("  Total: {:.3f}s, learned {} constants".format(total0, len(known)))

    # Perm 1: warm
    print("\n=== EXPLORER: Permutation 1 (warm — {} constants known) ===".format(len(known)))
    t0 = time.time()
    r1, known = explore(chain1, known)
    total1 = time.time() - t0
    print("  Total: {:.3f}s".format(total1))

    # Summary
    print("\n=== RESULTS ===")
    print("  Cold total:  {:.3f}s".format(total0))
    print("  Warm total:  {:.3f}s".format(total1))
    if total1 > 0:
        print("  Speedup:     {:.0f}x".format(total0 / total1))

    print("\n=== SECURITY MODEL ===")
    print("  Without hint: {:.1e} attempts — INFEASIBLE".format(
        hint_space * const_space * nonce_space * 6))
    print("  With hint:    {:.1e} attempts — seconds (cold unlock)".format(
        const_space * nonce_space * 6))
    print("  Known const:  {:.1e} attempts — instant (warm re-encounter)".format(
        nonce_space * 6))
    print()
    print("  hint (8-digit):     gates sequential access — must solve box N to reach N+1")
    print("  constant (4-digit): per-profile fingerprint — carry across permutations")
    print("  nonce (tunable):    per-entry random — single knob for difficulty")
    print("  ordering (6):       randomized key component order — prevents optimization")


if __name__ == "__main__":
    main()
