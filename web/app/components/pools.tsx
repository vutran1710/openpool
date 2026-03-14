export default function Pools() {
  return (
    <section id="pools" className="px-6 py-20">
      <div className="mx-auto max-w-5xl">
        <h2 className="mb-2 text-3xl font-bold">Pools</h2>
        <p className="mb-8 text-gray-600">
          Pools are GitHub repos that act as dating communities — like Discord
          servers, but fully decentralized. Anyone can create one.
        </p>

        <div className="grid gap-6 md:grid-cols-3">
          <div className="rounded-xl border border-violet-100 bg-white p-6">
            <h3 className="mb-2 font-semibold text-violet-800">Create</h3>
            <p className="mb-4 text-sm leading-relaxed text-gray-600">
              Spin up a GitHub repo, attach a fine-grained PAT and a Telegram
              bot token, then register it in the pool registry.
            </p>
            <div className="code-block px-4 py-3 text-xs">
              <span className="prompt">$</span>{" "}
              <span className="command">dating pool create my-pool ...</span>
            </div>
          </div>

          <div className="rounded-xl border border-violet-100 bg-white p-6">
            <h3 className="mb-2 font-semibold text-violet-800">Join</h3>
            <p className="mb-4 text-sm leading-relaxed text-gray-600">
              Browse the registry, pick a pool, and join. This opens a PR that
              the pool operator reviews and approves.
            </p>
            <div className="code-block px-4 py-3 text-xs">
              <span className="prompt">$</span>{" "}
              <span className="command">dating pool join my-pool</span>
            </div>
          </div>

          <div className="rounded-xl border border-violet-100 bg-white p-6">
            <h3 className="mb-2 font-semibold text-violet-800">Match</h3>
            <p className="mb-4 text-sm leading-relaxed text-gray-600">
              Inside a pool, likes are Pull Requests. When someone accepts your
              interest, the PR merges and you are matched.
            </p>
            <div className="code-block px-4 py-3 text-xs">
              <span className="prompt">$</span>{" "}
              <span className="command">dating like abc12</span>
            </div>
          </div>
        </div>

        <p className="mt-6 text-sm text-gray-500">
          Default registry:{" "}
          <a
            href="https://github.com/vutran1710/dating-pool-registry"
            target="_blank"
            rel="noopener noreferrer"
            className="text-violet-600 underline hover:text-violet-800"
          >
            vutran1710/dating-pool-registry
          </a>
        </p>
      </div>
    </section>
  );
}
