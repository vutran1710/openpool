import { Users, ArrowRight, Heart } from "lucide-react";

const steps = [
  {
    Icon: Users,
    title: "Create a pool",
    cmd: "dating pool create ...",
    strokeWidth: 1.3,
  },
  {
    Icon: ArrowRight,
    title: "People join",
    cmd: "dating pool join",
    strokeWidth: 2,
  },
  {
    Icon: Heart,
    title: "Matches happen",
    cmd: "dating like abc12",
    strokeWidth: 1.5,
  },
];

export default function Pools() {
  return (
    <section id="pools" className="border-t border-[var(--border)] px-6 py-24">
      <div className="mx-auto max-w-5xl">
        <div className="mb-12 text-center">
          <p className="font-handwritten text-3xl text-[var(--pink)]">
            how it works
          </p>
        </div>

        <div className="flex flex-col items-center gap-4 md:flex-row md:gap-0">
          {steps.map((step, i) => (
            <div key={step.title} className="flex w-full flex-1 flex-col items-center md:flex-row">
              <div className="hover-lift flex w-full flex-col items-center gap-3 rounded-xl border border-[var(--border)] bg-[var(--bg-surface)] p-6 text-center">
                <step.Icon
                  className="h-7 w-7 text-[var(--violet-400)]"
                  strokeWidth={step.strokeWidth}
                />
                <span className="text-sm font-semibold text-[var(--text)]">
                  {step.title}
                </span>
                <code className="text-xs text-[var(--text-dim)]">
                  {step.cmd}
                </code>
              </div>
              {i < steps.length - 1 && (
                <div className="hidden text-[var(--border)] md:block md:px-4">
                  <ArrowRight className="h-5 w-5" strokeWidth={1} />
                </div>
              )}
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
