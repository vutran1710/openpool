import {
  Boxes,
  GitPullRequestArrow,
  KeyRound,
  Compass,
  MessageCircleHeart,
  Sparkles,
} from "lucide-react";

const concepts = [
  {
    Icon: Boxes,
    label: "Decentralized",
    annotation: "no servers, ever",
    strokeWidth: 1.2,
  },
  {
    Icon: GitPullRequestArrow,
    label: "Git-native",
    annotation: "PRs are actions",
    strokeWidth: 1.8,
  },
  {
    Icon: KeyRound,
    label: "Encrypted",
    annotation: "your keys, your data",
    strokeWidth: 1.5,
  },
  {
    Icon: Compass,
    label: "Discoverable",
    annotation: "browse pools",
    strokeWidth: 1.3,
  },
  {
    Icon: MessageCircleHeart,
    label: "Real-time chat",
    annotation: "invisible transport",
    strokeWidth: 1.6,
  },
  {
    Icon: Sparkles,
    label: "Open source",
    annotation: "fork it, own it",
    strokeWidth: 2,
  },
];

export default function Concepts() {
  return (
    <section className="border-t border-[var(--border)] px-6 py-24">
      <div className="mx-auto max-w-5xl">
        <div className="grid grid-cols-2 gap-6 md:grid-cols-3">
          {concepts.map((c, i) => (
            <div
              key={c.label}
              className={`animate-fade-up stagger-${i + 1} hover-lift group flex flex-col items-center gap-3 rounded-xl border border-[var(--border)] bg-[var(--bg-surface)] p-8 text-center transition-all hover:border-[var(--violet-500)]`}
            >
              <c.Icon
                className="h-8 w-8 text-[var(--violet-400)] transition-colors group-hover:text-[var(--pink)]"
                strokeWidth={c.strokeWidth}
              />
              <span className="text-sm font-semibold tracking-wide text-[var(--text)]">
                {c.label}
              </span>
              <span className="font-handwritten text-lg text-[var(--text-dim)]">
                {c.annotation}
              </span>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
