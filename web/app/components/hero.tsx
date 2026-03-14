export default function Hero() {
  return (
    <section className="noise relative min-h-screen overflow-hidden">
      <div className="gradient-mesh grid-bg absolute inset-0" />

      <div className="relative mx-auto flex min-h-screen max-w-6xl flex-col items-center justify-center px-6 pt-16">
        <div className="animate-fade-up stagger-1 mb-6">
          <span className="font-handwritten text-2xl text-[var(--pink)]">
            for those who prefer
          </span>
        </div>

        <h1 className="animate-fade-up stagger-2 mb-2 text-center text-6xl font-extrabold tracking-tight md:text-8xl">
          <span className="glow gradient-text">dating</span>
          <span className="text-[var(--text-dim)]">.</span>
          <span className="text-[var(--text)]">dev</span>
        </h1>

        <p className="animate-fade-up stagger-3 mb-12 max-w-md text-center text-[var(--text-dim)]">
          Decentralized dating from your terminal. No servers.
          <br />
          Just GitHub repos, Pull Requests, and the command line.
        </p>

        <div className="animate-fade-up stagger-4 mb-6 w-full max-w-xl">
          <div className="terminal-border">
            <div className="terminal-header">
              <div className="terminal-dot bg-[#ff5f57]" />
              <div className="terminal-dot bg-[#febc2e]" />
              <div className="terminal-dot bg-[#28c840]" />
              <span className="ml-2 text-xs text-[var(--text-dim)]">
                terminal
              </span>
            </div>
            <div className="space-y-1 px-4 py-4 text-sm">
              <div>
                <span className="text-[var(--pink)]">$</span>{" "}
                <span className="text-[var(--violet-300)]">
                  curl -sSL https://dating.dev/install.sh | sh
                </span>
              </div>
              <div className="text-[var(--text-dim)]">
                Installing dating v0.1.0...
              </div>
              <div className="text-[var(--green)]">✓ Installed to /usr/local/bin/dating</div>
              <div className="mt-2">
                <span className="text-[var(--pink)]">$</span>{" "}
                <span className="text-[var(--violet-300)]">dating</span>
              </div>
              <div className="text-[var(--pink)]">♥ dating v0.1.0</div>
              <div className="text-[var(--text-dim)]">
                → Get Started{" "}
                <span className="font-handwritten text-base text-[var(--amber)]">
                  ← you are here
                </span>
              </div>
              <div className="text-[var(--text-dim)]">  Pools</div>
              <div className="text-[var(--text-dim)]">  Discover</div>
              <div className="text-[var(--text-dim)]">  Matches</div>
              <div className="text-[var(--text-dim)]">  Profile</div>
              <div className="mt-1">
                <span className="text-[var(--text-dim)]">
                  ↑/↓ navigate  enter select  q quit
                </span>
                <span className="blink text-[var(--violet-400)]">▌</span>
              </div>
            </div>
          </div>
        </div>

        <div className="animate-fade-up stagger-5 flex items-center gap-4 text-sm">
          <a
            href="https://github.com/vutran1710/dating-dev/releases"
            className="rounded-lg bg-[var(--pink)] px-5 py-2.5 font-semibold text-[var(--bg)] transition-all hover:shadow-[0_0_20px_var(--pink-dim)]"
          >
            Download Binary
          </a>
          <a
            href="/docs"
            className="rounded-lg border border-[var(--border)] px-5 py-2.5 text-[var(--text-dim)] transition-all hover:border-[var(--violet-400)] hover:text-[var(--violet-300)]"
          >
            Read the Docs →
          </a>
        </div>
      </div>
    </section>
  );
}
