export default function Install() {
  return (
    <section id="install" className="px-6 py-20">
      <div className="mx-auto max-w-5xl">
        <h2 className="mb-2 text-3xl font-bold">Install</h2>
        <p className="mb-8 text-gray-600">
          Get started in seconds. Requires Go 1.22+.
        </p>

        <div className="grid gap-6 md:grid-cols-2">
          <div className="rounded-xl border border-violet-100 bg-white p-6">
            <h3 className="mb-3 text-sm font-semibold uppercase tracking-wide text-violet-600">
              From source
            </h3>
            <div className="code-block px-5 py-4 text-sm">
              <div>
                <span className="prompt">$</span>{" "}
                <span className="command">
                  git clone https://github.com/vutran1710/dating-dev
                </span>
              </div>
              <div>
                <span className="prompt">$</span>{" "}
                <span className="command">cd dating-dev</span>
              </div>
              <div>
                <span className="prompt">$</span>{" "}
                <span className="command">make cli</span>
              </div>
              <div>
                <span className="prompt">$</span>{" "}
                <span className="command">./bin/dating</span>
              </div>
            </div>
          </div>

          <div className="rounded-xl border border-violet-100 bg-white p-6">
            <h3 className="mb-3 text-sm font-semibold uppercase tracking-wide text-violet-600">
              Go install
            </h3>
            <div className="code-block px-5 py-4 text-sm">
              <div>
                <span className="prompt">$</span>{" "}
                <span className="command">
                  go install github.com/vutran1710/dating-dev/cmd/dating@latest
                </span>
              </div>
              <div>
                <span className="prompt">$</span>{" "}
                <span className="command">dating</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
