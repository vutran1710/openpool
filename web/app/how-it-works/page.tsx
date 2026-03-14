import Image from "next/image";
import Nav from "../components/nav";
import Footer from "../components/footer";
import { Terminal, Search, Heart, MessageCircleHeart, Sparkles, GitPullRequestArrow } from "lucide-react";

const steps = [
  {
    number: "01",
    title: "Launch",
    tagline: "open the app, see your world",
    description:
      "Run dating and the interactive TUI opens. Navigate with arrow keys, select with enter, type / for commands. Everything from one screen.",
    screenshot: "/screenshots/tui-home.png",
    Icon: Terminal,
    strokeWidth: 1.3,
  },
  {
    number: "02",
    title: "Discover",
    tagline: "scroll through profiles",
    description:
      "Profiles appear as cards. See their city, interests, bio. Like someone with [l], skip with [s]. Each like creates a Pull Request on the pool's GitHub repo.",
    screenshot: "/screenshots/tui-discover.png",
    Icon: Search,
    strokeWidth: 1.5,
  },
  {
    number: "03",
    title: "Match",
    tagline: "when they like you back",
    description:
      "When both sides express interest, the PR merges automatically. A match directory is created in the repo. Fully auditable — every match is a git commit.",
    screenshot: "/screenshots/tui-match.png",
    Icon: GitPullRequestArrow,
    strokeWidth: 1.8,
  },
  {
    number: "04",
    title: "Chat",
    tagline: "talk, right here",
    description:
      "Messages stream in real-time through the Telegram bot — but you never leave the terminal. Type, send, scroll back. Use /exit to leave, /profile to peek.",
    screenshot: "/screenshots/tui-chat.png",
    Icon: MessageCircleHeart,
    strokeWidth: 1.5,
  },
];

const principles = [
  {
    Icon: Heart,
    title: "Pools are communities",
    desc: "Each pool is a GitHub repo. Anyone can create one. Like Discord servers, but decentralized.",
    strokeWidth: 1.5,
  },
  {
    Icon: GitPullRequestArrow,
    title: "Likes are Pull Requests",
    desc: "Every interest is a PR. Both sides approve, it merges — that merge is your match.",
    strokeWidth: 1.8,
  },
  {
    Icon: Sparkles,
    title: "No servers, no databases",
    desc: "GitHub repos store all state. Telegram relays chat. The CLI is the only interface. Zero infrastructure.",
    strokeWidth: 2,
  },
];

export default function HowItWorks() {
  return (
    <>
      <Nav />
      <div className="pt-16">
        <section className="noise relative px-6 py-20">
          <div className="gradient-mesh absolute inset-0" />
          <div className="relative mx-auto max-w-4xl text-center">
            <p className="font-handwritten mb-4 text-2xl text-[var(--pink)]">
              the journey
            </p>
            <h1 className="mb-4 text-5xl font-extrabold tracking-tight md:text-6xl">
              <span className="gradient-text">How it works</span>
            </h1>
            <p className="mx-auto max-w-md text-[var(--text-dim)]">
              From install to first conversation — four steps, all in your terminal.
            </p>
          </div>
        </section>

        <section className="px-6 py-8">
          <div className="mx-auto max-w-5xl space-y-24">
            {steps.map((step, i) => (
              <div
                key={step.number}
                className={`flex flex-col items-center gap-10 md:flex-row ${
                  i % 2 === 1 ? "md:flex-row-reverse" : ""
                }`}
              >
                <div className="flex-1">
                  <div className="mb-4 flex items-center gap-3">
                    <span className="text-3xl font-extrabold text-[var(--pink)] opacity-30">
                      {step.number}
                    </span>
                    <step.Icon
                      className="h-6 w-6 text-[var(--violet-400)]"
                      strokeWidth={step.strokeWidth}
                    />
                  </div>
                  <h2 className="mb-1 text-3xl font-bold text-[var(--text)]">
                    {step.title}
                  </h2>
                  <p className="font-handwritten mb-4 text-xl text-[var(--pink)]">
                    {step.tagline}
                  </p>
                  <p className="leading-relaxed text-[var(--text-dim)]">
                    {step.description}
                  </p>
                </div>
                <div className="w-full flex-1">
                  <div className="overflow-hidden rounded-xl border border-[var(--border)] shadow-[0_0_40px_rgba(139,92,246,0.08)]">
                    <Image
                      src={step.screenshot}
                      alt={`TUI ${step.title} screen`}
                      width={800}
                      height={450}
                      className="w-full"
                    />
                  </div>
                </div>
              </div>
            ))}
          </div>
        </section>

        <section className="border-t border-[var(--border)] px-6 py-20">
          <div className="mx-auto max-w-4xl">
            <p className="font-handwritten mb-10 text-center text-2xl text-[var(--pink)]">
              under the hood
            </p>
            <div className="grid gap-8 md:grid-cols-3">
              {principles.map((p) => (
                <div
                  key={p.title}
                  className="hover-lift rounded-xl border border-[var(--border)] bg-[var(--bg-surface)] p-6"
                >
                  <p.Icon
                    className="mb-3 h-6 w-6 text-[var(--violet-400)]"
                    strokeWidth={p.strokeWidth}
                  />
                  <h3 className="mb-2 text-sm font-bold text-[var(--text)]">
                    {p.title}
                  </h3>
                  <p className="text-xs leading-relaxed text-[var(--text-dim)]">
                    {p.desc}
                  </p>
                </div>
              ))}
            </div>
          </div>
        </section>
      </div>
      <Footer />
    </>
  );
}
