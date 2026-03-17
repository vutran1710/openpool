# Editable Profile

## Problem

The profile screen is view-only. Users can't edit their profile after joining a pool. Editing should re-encrypt and submit a registration issue to update the `.bin` file.

## Design

### Edit Mode

Press `e` on profile screen → step-by-step edit (same pattern as join flow):

1. **Interests** — tag input, comma-separated, backspace removes last, ctrl+d advances. Pre-filled from current profile.
2. **Intent** — checkbox multi-select (dating, friendship, yolo, networking). Pre-filled.
3. **Gender Target** — checkbox multi-select (men, women, non-binary, dev). Pre-filled.
4. **About** — textarea, 500 char limit, ctrl+d submits. Pre-filled.
5. **Confirm** — preview changes, enter to submit, esc to cancel.

Only user-entered fields are editable. GitHub-sourced fields (name, bio, location, avatar, website, social) are not editable here.

### On Submit

1. Save to `~/.dating/profile.json` (global) + `~/.dating/pools/{name}/profile.json` (per-pool)
2. Re-encrypt profile blob to operator pubkey (NaCl box)
3. Create registration Issue on pool repo (same format as join — title "Profile Update: {hash[:8]}", body with blob + pubkey + signature + identity proof)
4. Toast "Profile updated — waiting for pool to process"
5. Return to view mode, rebuild render cache

### Shared Registration Service

Extract encrypt+issue logic from join flow into a reusable function:

```go
// internal/cli/svc/registration.go

// SubmitProfileToPool encrypts the profile and creates a registration issue.
func SubmitProfileToPool(ctx context.Context, poolRepo, operatorPubKey, token string,
    profile gh.DatingProfile, userHash string,
    pub ed25519.PublicKey, priv ed25519.PrivateKey) (issueNum int, err error)
```

Steps inside:
1. Marshal profile to JSON
2. Pack into `.bin` format: `[32B user pubkey][profile encrypted to operator]`
3. Sign the blob
4. Create identity proof (encrypted user_id for operator)
5. Submit as registration Issue via `pool.RegisterUserViaIssue()`

Both join flow and profile edit call this function. The Pool Action processes it identically — if the user hash already exists, it overwrites the `.bin` file.

### Issue Title

- Join: `"Registration Request"` (existing)
- Update: `"Profile Update: {hash[:8]}"` with label `["profile-update"]`

The Action can distinguish by title/label and handle accordingly (same result — commit `.bin` file).

## File Changes

| File | Changes |
|------|---------|
| `internal/cli/svc/registration.go` | Create — shared `SubmitProfileToPool` function |
| `internal/cli/tui/screens/profile.go` | Add edit mode: step-by-step inputs, confirm, submit |
| `internal/cli/tui/screens/join.go` | Refactor to call shared `SubmitProfileToPool` |
| `internal/cli/tui/app.go` | Handle `ProfileUpdateMsg` from profile screen |
| `internal/github/pool.go` | Add `UpdateProfileViaIssue` (or reuse `RegisterUserViaIssue` with different title) |

## Testing

- Profile screen enters edit mode on `e`
- Each step pre-fills from current profile
- Esc cancels edit, returns to view mode
- Submit saves locally + creates registration issue
- Profile view refreshes after save
- Join flow still works after refactoring to shared service
