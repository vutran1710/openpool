# Pool Schema (pool.yaml)

## Goal

Single YAML file defining pool metadata, profile attributes, roles, and ranking. Replaces `pool.json`. The foundation for TUI rendering, profile validation, indexing, and future matching/VDF.

## Format

```yaml
name: <3 dating
description: terminal-native dating for developers
relay_url: wss://relay.example.com
operator_public_key: c251e2cf...

profile:
  age:
    type: range
    min: 18
    max: 100
  interests:
    type: multi
    values: ./values/interests.txt
  about:
    type: text
    required: false
  phone:
    type: text
    visibility: private
  social:
    type: text
    visibility: private
    required: false

roles:
  - man
  - woman

matching:
  # TBD — designed together with VDF bucketing
```

## Attribute Types

| Type | Input | TUI Component | Example |
|------|-------|---------------|---------|
| `enum` | Single value from list | Radio select | gender: male/female/nb |
| `multi` | Multiple values from list | Checkbox group | interests: hiking, coding |
| `range` | Number within min/max | Number input | age: 18-100 |
| `text` | Free text | Text input | about, phone |

## Attribute Properties

| Property | Default | Description |
|----------|---------|-------------|
| `type` | required | enum, multi, range, text |
| `required` | `true` | Must be filled to register |
| `visibility` | `public` | `public` (indexed, shown in discovery) or `private` (only revealed after match) |
| `values` | — | For enum/multi: inline list, comma-separated string, or file path |
| `min`, `max` | — | For range: bounds |

## Values Format

Values can be specified three ways:

**Inline list:**
```yaml
values:
  - hiking
  - coding
  - music
```

**Comma-separated:**
```yaml
values: yes, no, hybrid
```

**File reference:**
```yaml
values: ./values/interests.txt
```

File format: one value per line, or comma-separated. Parser splits on commas and newlines, trims whitespace.

## Roles

### Simple (shared profile, role is just a name)

```yaml
profile:
  age:
    type: range
    min: 18
    max: 100
  interests:
    type: multi
    values: hiking, coding, music

roles:
  - man
  - woman
```

All roles share the same profile attributes. Role defines identity (man matches with woman).

### Asymmetric (role-specific fields)

```yaml
profile:
  location:
    type: text

roles:
  employer:
    company:
      type: text
    salary_range:
      type: range
      min: 0
      max: 500000
  candidate:
    skills:
      type: multi
      values: ./values/skills.txt
    experience:
      type: range
      min: 0
      max: 30
```

Each role inherits `profile` attributes plus its own. When `roles` is a map (not a list), roles have different fields.

## Visibility & Privacy

- `public` attributes go into `index.db` — visible in discovery
- `private` attributes stay in `.bin` — only revealed after mutual match via encrypted chat
- TUI marks private fields with 🔒 icon

## TUI Rendering

The TUI generates profile forms dynamically from `pool.yaml`:

```
┌─ Profile: <3 dating ──────────────────────┐
│                                            │
│  ── Public ──                              │
│  Age          [ 28 ]                       │
│  Interests    ☑ hiking  ☐ coding  ☑ music │
│                                            │
│  ── Public (optional) ──                   │
│  About        [ _________________ ]        │
│                                            │
│  ── Private 🔒 ──                          │
│  Phone        [ _________________ ]        │
│                                            │
│  ── Private 🔒 (optional) ──              │
│  Social       [ _________________ ]        │
│                                            │
│           [ Submit ]                       │
└────────────────────────────────────────────┘
```

Grouped by visibility + optionality. Component type determined by attribute type.

## Onboarding Journey

The TUI onboarding flow is driven by `pool.yaml`. Steps:

### Step 1: Role Selection (if multiple roles)

If `roles` is a list with > 1 item, or a map with > 1 key:

```
┌─ Join: <3 dating ─────────────────────────┐
│                                            │
│  What are you?                             │
│                                            │
│  ● man                                     │
│  ○ woman                                   │
│                                            │
│           [ Next ]                         │
└────────────────────────────────────────────┘
```

If only one role → skip this step.

### Step 2: Base Profile (shared required fields)

Fill in all `required: true` fields from `profile:`:

```
┌─ Profile ─────────────────────────────────┐
│                                            │
│  Age          [ 28 ]                       │
│  Interests    ☑ hiking  ☐ coding  ☑ music │
│  Phone        [ _________ ]  🔒           │
│                                            │
│           [ Next ]                         │
└────────────────────────────────────────────┘
```

Optional fields shown but not enforced. Private fields marked with 🔒.

### Step 3: Role-Specific Fields (if any)

If the selected role has additional attributes (asymmetric roles):

```
┌─ Employer Details ────────────────────────┐
│                                            │
│  Company      [ _________________ ]        │
│  Salary Range [ 80000 ] - [ 120000 ]       │
│  Remote       ● yes  ○ no  ○ hybrid       │
│                                            │
│           [ Submit ]                       │
└────────────────────────────────────────────┘
```

If roles are a simple list (no role-specific fields) → skip this step, go straight to submit.

### Summary

```
Roles = 1           → Step 2 → Submit
Roles > 1 (list)    → Step 1 → Step 2 → Submit
Roles > 1 (map)     → Step 1 → Step 2 → Step 3 → Submit
```

## Validation

### Client-side (UX)

Before submitting registration:
- All `required: true` attributes must be filled
- `enum` values must be from the allowed list
- `range` values must be within min/max
- `text` values must not exceed payload limits

### Action-side (trust boundary)

`action-tool register` validates the same rules. Rejects + locks issues with invalid profiles.

### Schema Changes (migration rules)

**Non-breaking (allowed):**
- Add optional field (`required: false`)
- Relax required → optional
- Add new enum/multi values
- Change matcher rules
- Change pool metadata
- Change visibility `public → private`

**Breaking (rejected):**
- Remove a field
- Add a required field
- Change field type
- Remove enum values in use
- Rename a field
- Change visibility `private → public` (privacy violation)

## Matching

TBD — designed together with VDF bucketing. Will be a `matching:` section in pool.yaml.

## Migration from Current System

| Current | New |
|---------|-----|
| `pool.json` (JSON) | `pool.yaml` (YAML) |
| Hardcoded schema in Go types | Dynamic from pool.yaml |
| `PoolSchema` / `SchemaField` structs | Parsed from YAML at runtime |
| Fixed 4 match modes | TBD (VDF-integrated) |
| `pool.json` + separate schema | Single `pool.yaml` |
