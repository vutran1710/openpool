"use client";

import { useEffect, useState } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import Nav from "../components/nav";
import Footer from "../components/footer";
import docsContent from "../../content/docs.md.json";

type TocEntry = { id: string; text: string; level: number };

function extractToc(markdown: string): TocEntry[] {
  const headingRegex = /^(#{2,3})\s+(.+)$/gm;
  const entries: TocEntry[] = [];
  let match;

  while ((match = headingRegex.exec(markdown)) !== null) {
    const level = match[1].length;
    const text = match[2].replace(/[`*]/g, "");
    const id = text
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/(^-|-$)/g, "");
    entries.push({ id, text, level });
  }

  return entries;
}

function slugify(text: string): string {
  return text
    .toLowerCase()
    .replace(/[`*]/g, "")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/(^-|-$)/g, "");
}

export default function DocsPage() {
  const [activeId, setActiveId] = useState("");
  const toc = extractToc(docsContent);

  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        const visible = entries.find((e) => e.isIntersecting);
        if (visible) setActiveId(visible.target.id);
      },
      { rootMargin: "-80px 0px -70% 0px" }
    );

    toc.forEach(({ id }) => {
      const el = document.getElementById(id);
      if (el) observer.observe(el);
    });

    return () => observer.disconnect();
  }, [toc]);

  return (
    <>
      <Nav />
      <div className="mx-auto flex max-w-6xl px-6 py-16">
        <aside className="sticky top-20 hidden h-fit w-56 shrink-0 lg:block">
          <p className="mb-3 text-xs font-semibold uppercase tracking-wider text-violet-500">
            On this page
          </p>
          <nav className="space-y-0.5 border-l border-violet-100">
            {toc.map((entry) => (
              <a
                key={entry.id}
                href={`#${entry.id}`}
                className={`block py-1 text-sm transition-colors ${
                  entry.level === 3 ? "pl-6" : "pl-3"
                } ${
                  activeId === entry.id
                    ? "border-l-2 border-violet-500 font-medium text-violet-700"
                    : "text-gray-500 hover:text-violet-600"
                }`}
              >
                {entry.text}
              </a>
            ))}
          </nav>
        </aside>

        <main className="min-w-0 flex-1 lg:pl-12">
          <article className="prose prose-violet max-w-none prose-headings:scroll-mt-20 prose-h2:border-b prose-h2:border-violet-100 prose-h2:pb-2 prose-h2:text-gray-900 prose-h3:text-violet-800 prose-a:text-violet-600 prose-code:rounded prose-code:bg-violet-50 prose-code:px-1.5 prose-code:py-0.5 prose-code:text-violet-700 prose-code:before:content-none prose-code:after:content-none prose-pre:bg-[#1e1b2e] prose-pre:text-[#e2e0f0] prose-th:text-left prose-table:text-sm">
            <ReactMarkdown
              remarkPlugins={[remarkGfm]}
              components={{
                h2: ({ children, ...props }) => {
                  const text = String(children);
                  const id = slugify(text);
                  return (
                    <h2 id={id} {...props}>
                      {children}
                    </h2>
                  );
                },
                h3: ({ children, ...props }) => {
                  const text = String(children);
                  const id = slugify(text);
                  return (
                    <h3 id={id} {...props}>
                      {children}
                    </h3>
                  );
                },
              }}
            >
              {docsContent}
            </ReactMarkdown>
          </article>
        </main>
      </div>
      <Footer />
    </>
  );
}
