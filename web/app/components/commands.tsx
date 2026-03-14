const commandGroups = [
  {
    title: "Authentication",
    description: "Create an account and manage your session.",
    commands: [
      {
        cmd: "dating auth register",
        desc: "Create a new account via GitHub or Google OAuth",
        flags: ["-p, --provider  Auth provider (github or google)"],
      },
      {
        cmd: "dating auth login",
        desc: "Sign in to an existing account",
        flags: ["-p, --provider  Auth provider (github or google)"],
      },
      {
        cmd: "dating auth logout",
        desc: "Sign out and clear local credentials",
      },
      {
        cmd: "dating auth whoami",
        desc: "Display your current user info",
      },
    ],
  },
  {
    title: "Discovery",
    description: "Find and explore profiles.",
    commands: [
      {
        cmd: "dating fetch",
        desc: "Discover a random profile",
        flags: ["--city     Filter by city", "--interest  Filter by interest"],
      },
      {
        cmd: "dating view <public_id>",
        desc: "View someone's public profile",
      },
    ],
  },
  {
    title: "Matching",
    description: "Express interest and see your matches.",
    commands: [
      {
        cmd: "dating like <public_id>",
        desc: "Like someone — if they like you back, it's a match",
      },
      {
        cmd: "dating matches",
        desc: "List all your current matches",
      },
    ],
  },
  {
    title: "Chat",
    description: "Real-time messaging with your matches.",
    commands: [
      {
        cmd: "dating chat <public_id>",
        desc: "Open a live chat session with a match",
      },
    ],
  },
  {
    title: "Commitment",
    description: "Formalize a relationship.",
    commands: [
      {
        cmd: "dating commit <match_id>",
        desc: "Propose a commitment to a match",
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
        desc: "Interactively edit your bio, city, interests",
      },
      {
        cmd: "dating profile sync",
        desc: "Sync your profile to the public GitHub repository",
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
