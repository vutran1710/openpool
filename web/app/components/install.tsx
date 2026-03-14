export default function Install() {
  return (
    <section id="install" className="relative border-t border-[var(--border)] px-6 py-24">
      <div className="mx-auto max-w-4xl text-center">
        <p className="font-handwritten mb-8 text-3xl text-[var(--pink)]">
          one command away
        </p>
        <div className="terminal-border mx-auto max-w-lg">
          <div className="terminal-header">
            <div className="terminal-dot bg-[#ff5f57]" />
            <div className="terminal-dot bg-[#febc2e]" />
            <div className="terminal-dot bg-[#28c840]" />
          </div>
          <div className="px-5 py-4 text-sm">
            <span className="text-[var(--pink)]">$</span>{" "}
            <span className="text-[var(--violet-300)]">
              curl -sSL https://dating.dev/install.sh | sh
            </span>
          </div>
        </div>
      </div>
    </section>
  );
}
