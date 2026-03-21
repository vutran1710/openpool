# Interest as Issues (instead of PRs)

## Status: Concept — next iteration

## Core Idea

Replace interest PRs with interest Issues. Simpler, no branch management, better search.

```
Current:  like → CreatePR (needs branch, title=match_hash, label=interest)
Proposed: like → CreateIssue (title=match_hash, label=interest)
```

## Benefits

- No branch management (no git operations for `like` command)
- Issues searchable by title: `gh issue list --search "in:title <match_hash>" --label interest`
- Lighter weight, simpler CLI code
- Registration and interest both use issues, distinguished by label

## Action Triggers

```yaml
# Registration
if: contains(github.event.issue.labels.*.name, 'registration')

# Interest
if: contains(github.event.issue.labels.*.name, 'interest')
```

## Requires

- `ListIssues` method on GitHubClient interface
- Update `like` command: `CreatePullRequest` → `CreateIssue`
- Update `matchcrypt match`: search issues instead of PRs
- Update Action templates: trigger on issue labels
- Update CLI matches/inbox: poll issues instead of PRs

## Open Questions

- Should interest issue title be the target's match_hash (same as current PR)?
- How to handle the `matchcrypt match` reciprocal search — title search vs label filter?
- Migration: close existing interest PRs?
