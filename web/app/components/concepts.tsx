const concepts = [
  {
    title: "Pools = GitHub Repos",
    description:
      "Each dating pool is a GitHub repository — like a Discord server anyone can create. No central server; the repos ARE the platform.",
    icon: "🏊",
  },
  {
    title: "Registry",
    description:
      "Pools register themselves in a public registry repo (default: vutran1710/dating-pool-registry). Users browse and join pools from there.",
    icon: "📋",
  },
  {
    title: "PR-based Matching",
    description:
      "Likes are Pull Requests. When someone accepts your interest, the PR merges — that merge is your match. Fully auditable on GitHub.",
    icon: "🔀",
  },
  {
    title: "Local ed25519 Identity",
    description:
      "Your identity is a locally generated ed25519 key pair. No OAuth, no accounts — just cryptographic keys you control.",
    icon: "🔑",
  },
  {
    title: "Telegram Chat Transport",
    description:
      "Matched users chat through a Telegram bot managed by the pool operator. The transport is invisible — you just use the CLI.",
    icon: "💬",
  },
  {
    title: "PR Templates for Monetization",
    description:
      "Pool operators customize PR templates to set admission requirements. This enables monetization, gating, and community curation.",
    icon: "📝",
  },
  {
    title: "GitHub Actions Automation",
    description:
      "Webhooks and GitHub Actions fire on PR merge, enabling automated match notifications, onboarding flows, and custom workflows.",
    icon: "⚡",
  },
  {
    title: "Commitment System",
    description:
      "Formalize relationships with a mutual commitment — a proposal/accept flow that creates a signed artifact in the pool repo.",
    icon: "💜",
  },
];

export default function Concepts() {
  return (
    <section id="concepts" className="px-6 py-20">
      <div className="mx-auto max-w-5xl">
        <h2 className="mb-2 text-3xl font-bold">Core Concepts</h2>
        <p className="mb-10 text-gray-600">
          A fully decentralized architecture built on GitHub.
        </p>

        <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
          {concepts.map((concept) => (
            <div
              key={concept.title}
              className="rounded-xl border border-violet-100 bg-white p-6 transition-shadow hover:shadow-md"
            >
              <div className="mb-3 text-2xl">{concept.icon}</div>
              <h3 className="mb-2 font-semibold text-violet-800">
                {concept.title}
              </h3>
              <p className="text-sm leading-relaxed text-gray-600">
                {concept.description}
              </p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
