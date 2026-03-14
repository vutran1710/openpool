import Nav from "./components/nav";
import Hero from "./components/hero";
import Concepts from "./components/concepts";
import Footer from "./components/footer";

export default function Home() {
  return (
    <>
      <Nav />
      <Hero />
      <section className="border-t border-[var(--border)] px-6 py-16">
        <div className="mx-auto max-w-md text-center">
          <p className="font-handwritten mb-4 text-2xl text-[var(--pink)]">
            curious?
          </p>
          <a
            href="/how-it-works"
            className="inline-block rounded-lg border border-[var(--border)] px-6 py-3 text-sm font-medium text-[var(--text)] transition-all hover:border-[var(--violet-500)] hover:shadow-[0_0_20px_rgba(139,92,246,0.1)]"
          >
            See how it works →
          </a>
        </div>
      </section>
      <Concepts />
      <Footer />
    </>
  );
}
