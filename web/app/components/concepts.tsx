const concepts = [
  {
    title: "Pseudonymous Identity",
    description:
      "You interact using a short public ID like 8ac21. Real identities stay private until you choose to share them.",
    icon: "🎭",
  },
  {
    title: "Client-side Encryption",
    description:
      "Sensitive profile data is encrypted locally before leaving your machine. Only you hold the keys.",
    icon: "🔐",
  },
  {
    title: "Public Artifacts on GitHub",
    description:
      "Public profiles are stored in a Git repository — transparent, auditable, and portable.",
    icon: "📦",
  },
  {
    title: "Stream-based Chat",
    description:
      "Conversations behave like log streams. Messages append in real-time, just like tailing a log.",
    icon: "📡",
  },
  {
    title: "Developer & Non-dev Friendly",
    description:
      "Sign up with GitHub (developers) or Google (everyone). The CLI is designed to feel natural for all users.",
    icon: "👥",
  },
  {
    title: "Commitment System",
    description:
      "Formalize relationships with a mutual commitment — a proposal/accept flow that creates a signed artifact.",
    icon: "💜",
  },
];

export default function Concepts() {
  return (
    <section id="concepts" className="px-6 py-20">
      <div className="mx-auto max-w-5xl">
        <h2 className="mb-2 text-3xl font-bold">Core Concepts</h2>
        <p className="mb-10 text-gray-600">
          The principles behind Dating CLI.
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
