export default function Hero() {
  return (
    <section className="hero-gradient px-6 pb-16 pt-24">
      <div className="mx-auto max-w-5xl text-center">
        <h1 className="mb-4 text-5xl font-bold tracking-tight md:text-6xl">
          <span className="gradient-text">Dating CLI</span>
        </h1>
        <p className="mx-auto mb-8 max-w-lg text-lg text-gray-600">
          A terminal-native dating platform. Discover, match, and chat — all
          from the command line.
        </p>
        <div className="flex items-center justify-center gap-4">
          <div className="code-block inline-flex items-center gap-2 px-5 py-3 text-sm">
            <span className="prompt">$</span>
            <span className="command">
              go install github.com/vutran1710/dating-dev/cmd/dating@latest
            </span>
          </div>
        </div>
        <div className="mt-4 flex items-center justify-center gap-4 text-sm">
          <a
            href="https://github.com/vutran1710/dating-dev/releases"
            className="gradient-bg rounded-lg px-5 py-2.5 font-medium text-white transition-opacity hover:opacity-90"
          >
            Download
          </a>
          <a
            href="https://github.com/vutran1710/dating-dev"
            className="rounded-lg border border-violet-200 bg-white px-5 py-2.5 font-medium text-violet-700 transition-colors hover:bg-violet-50"
          >
            View on GitHub
          </a>
        </div>
      </div>
    </section>
  );
}
