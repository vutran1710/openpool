export default function Nav() {
  return (
    <nav className="sticky top-0 z-50 border-b border-violet-100 bg-white/80 backdrop-blur-md">
      <div className="mx-auto flex max-w-5xl items-center justify-between px-6 py-4">
        <a href="/" className="flex items-center gap-2 text-lg font-semibold">
          <span className="gradient-text">♥ dating</span>
        </a>
        <div className="flex items-center gap-6 text-sm font-medium text-gray-600">
          <a href="/pools" className="hover:text-violet-600 transition-colors">
            Pools
          </a>
          <a href="/docs" className="hover:text-violet-600 transition-colors">
            Docs
          </a>
          <a
            href="https://github.com/vutran1710/dating-dev"
            target="_blank"
            rel="noopener noreferrer"
            className="rounded-lg bg-violet-600 px-4 py-2 text-white hover:bg-violet-700 transition-colors"
          >
            GitHub
          </a>
        </div>
      </div>
    </nav>
  );
}
