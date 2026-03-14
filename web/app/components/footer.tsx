import { Github, Heart } from "lucide-react";

export default function Footer() {
  return (
    <footer className="border-t border-[var(--border)] px-6 py-8">
      <div className="mx-auto flex max-w-5xl flex-col items-center justify-between gap-4 md:flex-row">
        <div className="flex items-center gap-2 text-xs text-[var(--text-dim)]">
          <Heart className="h-3 w-3 text-[var(--pink)]" fill="currentColor" strokeWidth={0} />
          <span>dating.dev</span>
        </div>
        <div className="flex gap-6 text-xs text-[var(--text-dim)]">
          <a
            href="https://github.com/vutran1710/dating-dev"
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-1.5 transition-colors hover:text-[var(--violet-400)]"
          >
            <Github className="h-3.5 w-3.5" strokeWidth={1.5} />
            Source
          </a>
          <a
            href="https://github.com/vutran1710/dating-dev/issues"
            target="_blank"
            rel="noopener noreferrer"
            className="transition-colors hover:text-[var(--violet-400)]"
          >
            Issues
          </a>
          <a href="/docs" className="transition-colors hover:text-[var(--violet-400)]">
            Docs
          </a>
        </div>
      </div>
    </footer>
  );
}
