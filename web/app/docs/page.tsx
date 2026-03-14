import Nav from "../components/nav";
import Footer from "../components/footer";

const sections = [
  {
    id: "architecture",
    title: "Architecture",
    content: `Dating CLI is fully decentralized. There are no servers — GitHub repos act as both database and state machine.

**Components:**
- **CLI** — the only user interface (Go binary)
- **Pool repos** — GitHub repos that hold profiles, indexes, matches
- **Registry repo** — where pools register for discovery
- **Telegram bot** — invisible chat transport between matched users

**Data flow:**
- Reads: CLI → GitHub API (profiles, indexes, match state)
- Writes: CLI → GitHub API (creates PRs for joins, likes, matches)
- Chat: CLI → Telegram Bot API → CLI`,
  },
  {
    id: "identity",
    title: "Identity & Keys",
    content: `Each user has a local ed25519 key pair generated at registration.

**Files:**
\`\`\`
~/.dating/
  keys/
    identity.pub    # ed25519 public key
    identity.key    # ed25519 private key (never leaves your machine)
  config.toml       # pools, active pool, user info
\`\`\`

**Public ID** — derived from first 5 hex chars of the public key (e.g. \`8ac21\`).

**Signing** — every action (like, join, profile update) is signed with your private key. The signature is included in the PR body so pool operators can verify authenticity.`,
  },
  {
    id: "pools",
    title: "Pools",
    content: `A pool is a GitHub repository that acts as a dating community.

**Repo structure:**
\`\`\`
pool-repo/
  pool.json                          # pool metadata
  users/{public_id}/public.json      # user profiles
  index/
    by-status/open/{public_id}       # symlinks to profiles
    by-city/{city}/{public_id}
    by-interest/{interest}/{public_id}
  matches/{hash}/                    # matched pairs
  commitments/{hash}.json            # commitment artifacts
  .github/
    PULL_REQUEST_TEMPLATE/
      join.md                        # join PR template (monetization)
      like.md                        # like PR template
    workflows/                       # automation on merge
\`\`\`

**Creating a pool:**
\`\`\`bash
dating pool create my-pool \\
  --repo owner/my-pool-repo \\
  --gh-token ghp_xxx \\
  --bot-token 123:ABC \\
  --registry-token ghp_yyy \\
  --desc "My dating community"
\`\`\`

This creates a PR to the registry. Once the registry maintainer merges it, the pool is live.`,
  },
  {
    id: "registry",
    title: "Registry",
    content: `The registry is a GitHub repo where pools register for discovery.

**Structure:**
\`\`\`
registry-repo/
  pools/{pool-name}/
    pool.json       # name, repo, description, created_at
    tokens.bin      # serialized GitHub PAT + Telegram bot token
\`\`\`

**Joining flow:**
1. \`dating pool browse\` — lists pools from the registry
2. \`dating pool join <name>\` — fetches pool config from registry, then creates a join PR to the pool repo
3. Pool operator reviews and merges

Anyone can run their own registry. The default is \`vutran1710/dating-pool-registry\`.

**Custom registry:**
\`\`\`bash
dating pool browse --registry your-org/your-registry
dating pool join my-pool --registry your-org/your-registry
\`\`\``,
  },
  {
    id: "matching",
    title: "Matching (PRs)",
    content: `Likes and matches use GitHub Pull Requests as the mechanism.

**Like flow:**
1. \`dating like 8ac21\` — CLI creates a PR on the pool repo
2. PR adds both profiles to \`matches/{hash}/\`
3. PR is labeled \`like:8ac21\` for inbox filtering
4. The other user sees it via \`dating inbox\`

**Accept flow:**
1. \`dating accept <pr_number>\` — merges the PR
2. Match directory is created on main branch
3. Both users can now chat

**Match hash** — deterministic SHA256 of both public IDs (canonically ordered), truncated to 12 chars. Prevents duplicate match directories.

**Why PRs?**
- Mutual consent (both parties involved)
- Full git history / audit trail
- PR comments as icebreakers
- GitHub Actions can automate post-match workflows
- Labels and templates enable custom admission criteria`,
  },
  {
    id: "chat",
    title: "Chat (Telegram)",
    content: `Chat uses a Telegram bot as invisible transport. Users never interact with Telegram directly.

**How it works:**
\`\`\`
CLI (user A) → Telegram Bot API → Bot stores message
CLI (user B) → Telegram Bot API → Bot delivers message
\`\`\`

**Setup:** The pool creator provides a Telegram bot token when registering the pool. It's distributed to users via the registry.

**In the CLI:**
\`\`\`bash
dating chat 8ac21
  dating:8ac21> hello!
  [14:22] 8ac21: hey back
  dating:8ac21> /exit
\`\`\`

**Chat commands:**
- \`/exit\` — leave the chat
- \`/history\` — view message history
- \`/profile\` — view the other person's profile`,
  },
  {
    id: "monetization",
    title: "Monetization (PR Templates)",
    content: `Pool operators and registry maintainers can monetize via GitHub PR templates.

**How it works:**
1. Operator creates \`.github/PULL_REQUEST_TEMPLATE/join.md\` in the pool repo
2. Template can include custom fields using \`{{ field_name }}\` placeholders
3. When a user runs \`dating pool join\`, the CLI fetches the template, displays requirements, and prompts the user to fill in fields
4. Filled template is included in the PR body

**Example template (\`join.md\`):**
\`\`\`markdown
## Join Requirements

To join this pool, you must be a GitHub Sponsor ($5/mo).
Sponsor link: https://github.com/sponsors/pool-owner

Sponsor username: {{ sponsor_username }}
Transaction date: {{ transaction_date }}
\`\`\`

**Automation:** Pool operators can set up GitHub Actions that verify payment/sponsorship before auto-merging the PR.

**Revenue model:** The pool operator keeps 100%. Zero platform cut.`,
  },
  {
    id: "commands",
    title: "CLI Reference",
    content: `**Authentication:**
\`\`\`bash
dating auth register      # create identity (ed25519 keys)
dating auth whoami        # show current identity
\`\`\`

**Pools:**
\`\`\`bash
dating pool browse        # browse registry
dating pool create <name> # register a new pool
dating pool join <name>   # join a pool (creates PR)
dating pool leave <name>  # leave a pool
dating pool list          # list joined pools
dating pool switch <name> # set active pool
\`\`\`

**Discovery:**
\`\`\`bash
dating fetch              # random profile from active pool
dating view <public_id>   # view a profile
\`\`\`

**Matching:**
\`\`\`bash
dating like <public_id>   # express interest (creates PR)
dating inbox              # view incoming interests
dating accept <pr_number> # accept interest (merge PR = match)
\`\`\`

**Chat:**
\`\`\`bash
dating chat <public_id>   # chat with a match
\`\`\`

**Commitment:**
\`\`\`bash
dating commit <public_id> # propose commitment
dating status             # check relationship status
\`\`\`

**Profile:**
\`\`\`bash
dating profile edit       # edit and publish profile
dating profile show       # show current profile
\`\`\``,
  },
  {
    id: "config",
    title: "Configuration",
    content: `All config lives in \`~/.dating/config.toml\`:

\`\`\`toml
[user]
public_id = "8ac21"
display_name = "alice"

[[pools]]
name = "berlin-singles"
repo = "owner/berlin-singles"
token = "github_pat_xxx"
bot_token = "123456:ABC"
status = "active"

[[pools]]
name = "tokyo-devs"
repo = "owner/tokyo-devs"
token = "github_pat_yyy"
bot_token = "789012:DEF"
status = "pending"

active_pool = "berlin-singles"
\`\`\`

**Environment variables:**
- \`DATING_CONFIG_DIR\` — override config directory (default: \`~/.dating\`)`,
  },
  {
    id: "contributing",
    title: "Contributing",
    content: `**Build from source:**
\`\`\`bash
git clone https://github.com/vutran1710/dating-dev
cd dating-dev
make build        # builds bin/dating
make test         # runs tests
make lint         # runs golangci-lint
\`\`\`

**Project structure:**
\`\`\`
cmd/dating/           # CLI entry point
internal/
  cli/                # CLI commands and TUI
  cli/config/         # ~/.dating config management
  cli/tui/            # Interactive TUI mode
  crypto/             # ed25519 key management
  github/             # GitHub API client, pool, registry
  telegram/           # Telegram bot client
web/                  # Next.js docs site
\`\`\`

**Tech stack:** Go, Cobra, Bubbletea, Lipgloss, ed25519`,
  },
];

function renderContent(content: string) {
  const parts = content.split(/(```[\s\S]*?```)/g);
  return parts.map((part, i) => {
    if (part.startsWith("```")) {
      const lines = part.split("\n");
      const lang = lines[0].replace("```", "").trim();
      const code = lines.slice(1, -1).join("\n");
      return (
        <div key={i} className="code-block my-4 px-5 py-4 text-sm">
          {lang && (
            <div className="mb-2 text-xs uppercase tracking-wide text-violet-400">
              {lang}
            </div>
          )}
          <pre className="whitespace-pre-wrap">
            <code>{code}</code>
          </pre>
        </div>
      );
    }

    const lines = part.split("\n").map((line, j) => {
      if (line.startsWith("**") && line.endsWith("**")) {
        return (
          <p key={j} className="mt-4 mb-1 font-semibold text-violet-800">
            {line.replace(/\*\*/g, "")}
          </p>
        );
      }
      if (line.startsWith("- ")) {
        const text = line.slice(2);
        const boldMatch = text.match(/^\*\*(.*?)\*\*(.*)/);
        if (boldMatch) {
          return (
            <li key={j} className="ml-4 text-gray-700">
              <strong className="text-violet-700">{boldMatch[1]}</strong>
              {boldMatch[2]}
            </li>
          );
        }
        return (
          <li key={j} className="ml-4 text-gray-700">
            {text}
          </li>
        );
      }
      if (line.match(/^\d+\. /)) {
        return (
          <li key={j} className="ml-4 list-decimal text-gray-700">
            {line.replace(/^\d+\. /, "")}
          </li>
        );
      }
      if (line.trim() === "") return <br key={j} />;
      return (
        <p key={j} className="text-gray-700 leading-relaxed">
          {line}
        </p>
      );
    });

    return <div key={i}>{lines}</div>;
  });
}

export default function DocsPage() {
  return (
    <>
      <Nav />
      <div className="mx-auto flex max-w-6xl px-6 py-16">
        <aside className="sticky top-20 hidden h-fit w-56 shrink-0 lg:block">
          <nav className="space-y-1">
            {sections.map((s) => (
              <a
                key={s.id}
                href={`#${s.id}`}
                className="block rounded-md px-3 py-1.5 text-sm text-gray-600 transition-colors hover:bg-violet-50 hover:text-violet-700"
              >
                {s.title}
              </a>
            ))}
          </nav>
        </aside>

        <main className="min-w-0 flex-1 lg:pl-12">
          <h1 className="mb-2 text-4xl font-bold">
            <span className="gradient-text">Documentation</span>
          </h1>
          <p className="mb-12 text-gray-500">
            Technical reference for developers building with and on Dating CLI.
          </p>

          <div className="space-y-16">
            {sections.map((s) => (
              <section key={s.id} id={s.id}>
                <h2 className="mb-4 border-b border-violet-100 pb-2 text-2xl font-bold text-gray-900">
                  {s.title}
                </h2>
                <div className="prose max-w-none">{renderContent(s.content)}</div>
              </section>
            ))}
          </div>
        </main>
      </div>
      <Footer />
    </>
  );
}
