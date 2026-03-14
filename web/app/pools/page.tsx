import Nav from "../components/nav";
import Footer from "../components/footer";

const availablePools = [
  {
    name: "dating-pool-registry",
    description: "The official default registry. Register your pool here.",
    repo: "vutran1710/dating-pool-registry",
    status: "Official",
  },
];

export default function PoolsPage() {
  return (
    <>
      <Nav />
      <div className="mx-auto max-w-5xl px-6 py-16">
        <h1 className="mb-2 text-4xl font-bold">
          <span className="gradient-text">Pool Registry</span>
        </h1>
        <p className="mb-12 text-gray-500">
          Browse pools, or create your own and register it.
        </p>

        <section className="mb-16">
          <h2 className="mb-6 text-2xl font-bold">Available Pools</h2>
          <div className="space-y-4">
            {availablePools.map((pool) => (
              <div
                key={pool.name}
                className="flex items-center justify-between rounded-xl border border-violet-100 bg-white p-6 transition-shadow hover:shadow-md"
              >
                <div>
                  <div className="flex items-center gap-3">
                    <h3 className="text-lg font-semibold text-violet-800">
                      {pool.name}
                    </h3>
                    <span className="rounded-full bg-violet-100 px-2.5 py-0.5 text-xs font-medium text-violet-700">
                      {pool.status}
                    </span>
                  </div>
                  <p className="mt-1 text-sm text-gray-600">
                    {pool.description}
                  </p>
                  <p className="mt-1 font-mono text-xs text-gray-400">
                    {pool.repo}
                  </p>
                </div>
                <div className="code-block shrink-0 px-4 py-2 text-xs">
                  <span className="prompt">$</span>{" "}
                  <span className="command">dating pool browse</span>
                </div>
              </div>
            ))}
          </div>
          <p className="mt-4 text-sm text-gray-400">
            More pools will appear here as they are registered. Run{" "}
            <code className="rounded bg-violet-50 px-1.5 py-0.5 text-violet-700">
              dating pool browse
            </code>{" "}
            to see the latest.
          </p>
        </section>

        <section className="mb-16">
          <h2 className="mb-6 text-2xl font-bold">Quick Start</h2>
          <div className="space-y-6">
            <Step
              number={1}
              title="Install the CLI"
              code="go install github.com/vutran1710/dating-dev/cmd/dating@latest"
            />
            <Step
              number={2}
              title="Create your identity"
              code="dating auth register"
            />
            <Step
              number={3}
              title="Browse available pools"
              code="dating pool browse"
            />
            <Step
              number={4}
              title="Join a pool"
              code="dating pool join <pool-name>"
              note="This creates a PR. The pool operator will review and approve."
            />
            <Step
              number={5}
              title="Start discovering"
              code="dating fetch"
              note="Once your join PR is merged, you can discover profiles."
            />
          </div>
        </section>

        <section className="mb-16">
          <h2 className="mb-6 text-2xl font-bold">Create Your Own Pool</h2>
          <p className="mb-6 text-gray-600">
            Anyone can create a pool. You need a GitHub repo and optionally a
            Telegram bot for chat.
          </p>

          <div className="space-y-6">
            <Step
              number={1}
              title="Create a GitHub repository"
              code="gh repo create my-dating-pool --public"
            />
            <Step
              number={2}
              title="Create a fine-grained GitHub PAT"
              note="Go to GitHub → Settings → Developer settings → Fine-grained tokens. Scope it to your pool repo with Pull Request (write) and Contents (write) permissions."
            />
            <Step
              number={3}
              title="(Optional) Create a Telegram bot"
              note="Message @BotFather on Telegram, run /newbot, and save the token."
            />
            <Step
              number={4}
              title="Register your pool"
              code={`dating pool create my-pool \\
  --repo your-name/my-dating-pool \\
  --gh-token github_pat_xxx \\
  --bot-token 123456:ABC \\
  --registry-token github_pat_yyy \\
  --desc "My awesome dating community"`}
              note="This creates a PR to the registry. Once merged, your pool is discoverable."
            />
            <Step
              number={5}
              title="(Optional) Add a PR template for monetization"
              code={`# .github/PULL_REQUEST_TEMPLATE/join.md
## Join Requirements

Sponsor link: https://github.com/sponsors/you

Sponsor username: {{ sponsor_username }}`}
              note="The CLI will prompt users to fill in these fields when joining."
            />
            <Step
              number={6}
              title="(Optional) Add GitHub Actions"
              note="Set up workflows that trigger on PR merge — verify payments, send welcome messages, update indexes."
            />
          </div>
        </section>

        <section>
          <h2 className="mb-6 text-2xl font-bold">Pool Management</h2>
          <div className="code-block px-6 py-5 text-sm leading-loose">
            <div>
              <span className="prompt">$</span>{" "}
              <span className="command">dating pool list</span>
            </div>
            <div className="output">
              {"  "}* berlin-singles{"  "}[active]{"  "}[chat]
            </div>
            <div className="output">
              {"    "}tokyo-devs{"      "}[pending]
            </div>
            <div className="mt-3">
              <span className="prompt">$</span>{" "}
              <span className="command">dating pool switch tokyo-devs</span>
            </div>
            <div className="success">{"  "}Active pool: tokyo-devs</div>
            <div className="mt-3">
              <span className="prompt">$</span>{" "}
              <span className="command">dating pool leave berlin-singles</span>
            </div>
            <div className="success">
              {"  "}Left &quot;berlin-singles&quot;
            </div>
          </div>
        </section>
      </div>
      <Footer />
    </>
  );
}

function Step({
  number,
  title,
  code,
  note,
}: {
  number: number;
  title: string;
  code?: string;
  note?: string;
}) {
  return (
    <div className="flex gap-4">
      <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-violet-100 text-sm font-bold text-violet-700">
        {number}
      </div>
      <div className="flex-1">
        <h3 className="font-semibold text-gray-900">{title}</h3>
        {code && (
          <div className="code-block mt-2 px-4 py-3 text-sm">
            <pre className="whitespace-pre-wrap">
              <span className="command">{code}</span>
            </pre>
          </div>
        )}
        {note && <p className="mt-2 text-sm text-gray-500">{note}</p>}
      </div>
    </div>
  );
}
