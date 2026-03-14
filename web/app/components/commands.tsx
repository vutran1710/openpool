const commandGroups = [
  {
    title: "Authentication",
    description: "Create and manage your local identity.",
    commands: [
      {
        cmd: "dating auth register",
        desc: "Create a new identity (generates ed25519 key pair)",
      },
      {
        cmd: "dating auth whoami",
        desc: "Show your current identity",
      },
    ],
  },
  {
    title: "Pools",
    description: "Browse, create, and join dating pools (GitHub repos).",
    commands: [
      {
        cmd: "dating pool browse",
        desc: "Browse available pools from a registry",
      },
      {
        cmd: "dating pool create <name>",
        desc: "Create and register a new pool",
        flags: [
          "--repo        GitHub repo for the pool",
          "--gh-token    Fine-grained GitHub PAT",
          "--bot-token   Telegram bot token",
          "--registry-token  Token for registry repo",
        ],
      },
      {
        cmd: "dating pool join <name>",
        desc: "Join a pool (creates a PR, pending operator approval)",
      },
      {
        cmd: "dating pool leave <name>",
        desc: "Leave a pool",
      },
      {
        cmd: "dating pool list",
        desc: "List joined pools with status (pending/active)",
      },
      {
        cmd: "dating pool switch <name>",
        desc: "Set the active pool",
      },
    ],
  },
  {
    title: "Discovery",
    description: "Find and explore profiles in your active pool.",
    commands: [
      {
        cmd: "dating fetch",
        desc: "Discover random profiles in the active pool",
      },
      {
        cmd: "dating view <public_id>",
        desc: "View someone's profile",
      },
    ],
  },
  {
    title: "Matching",
    description: "Likes are Pull Requests. Matches are merged PRs.",
    commands: [
      {
        cmd: "dating like <public_id>",
        desc: "Express interest (creates a PR)",
      },
      {
        cmd: "dating inbox",
        desc: "View incoming interests (open PRs targeting you)",
      },
      {
        cmd: "dating accept <pr_number>",
        desc: "Accept an interest (merges the PR = match)",
      },
    ],
  },
  {
    title: "Chat",
    description: "Message your matches via Telegram bot transport.",
    commands: [
      {
        cmd: "dating chat <public_id>",
        desc: "Chat with a match (via Telegram bot, invisible to users)",
      },
    ],
  },
  {
    title: "Commitment",
    description: "Formalize a relationship.",
    commands: [
      {
        cmd: "dating commit <public_id>",
        desc: "Propose commitment to a match",
      },
      {
        cmd: "dating status",
        desc: "Check your current relationship status",
      },
    ],
  },
  {
    title: "Profile",
    description: "Manage your dating profile.",
    commands: [
      {
        cmd: "dating profile edit",
        desc: "Edit and publish your profile to the active pool",
      },
      {
        cmd: "dating profile show",
        desc: "Show your current profile",
      },
    ],
  },
];

function CommandCard({
  cmd,
  desc,
  flags,
}: {
  cmd: string;
  desc: string;
  flags?: string[];
}) {
  return (
    <div className="rounded-lg border border-gray-100 bg-white p-4 transition-shadow hover:shadow-md">
      <code className="text-sm font-semibold text-violet-700">{cmd}</code>
      <p className="mt-1.5 text-sm text-gray-600">{desc}</p>
      {flags && flags.length > 0 && (
        <div className="mt-3 rounded-md bg-gray-50 px-3 py-2">
          {flags.map((flag, i) => (
            <div key={i} className="font-mono text-xs text-gray-500">
              {flag}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

export default function Commands() {
  return (
    <section id="commands" className="bg-violet-50/50 px-6 py-20">
      <div className="mx-auto max-w-5xl">
        <h2 className="mb-2 text-3xl font-bold">Commands</h2>
        <p className="mb-10 text-gray-600">
          Everything you can do from the terminal.
        </p>

        <div className="space-y-12">
          {commandGroups.map((group) => (
            <div key={group.title}>
              <h3 className="mb-1 text-xl font-semibold text-violet-800">
                {group.title}
              </h3>
              <p className="mb-4 text-sm text-gray-500">{group.description}</p>
              <div className="grid gap-3 md:grid-cols-2">
                {group.commands.map((command) => (
                  <CommandCard key={command.cmd} {...command} />
                ))}
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
