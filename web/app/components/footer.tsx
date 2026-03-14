export default function Footer() {
  return (
    <footer className="border-t border-violet-100 bg-white px-6 py-10">
      <div className="mx-auto flex max-w-5xl flex-col items-center justify-between gap-4 md:flex-row">
        <div className="flex items-center gap-2 text-sm text-gray-500">
          <span className="gradient-text font-semibold">♥ dating</span>
          <span>— terminal-native dating</span>
        </div>
        <div className="flex gap-6 text-sm text-gray-500">
          <a
            href="https://github.com/vutran1710/dating-dev"
            target="_blank"
            rel="noopener noreferrer"
            className="hover:text-violet-600 transition-colors"
          >
            GitHub
          </a>
          <a
            href="https://github.com/vutran1710/dating-dev/issues"
            target="_blank"
            rel="noopener noreferrer"
            className="hover:text-violet-600 transition-colors"
          >
            Issues
          </a>
        </div>
      </div>
    </footer>
  );
}
