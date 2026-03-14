import { Heart } from "lucide-react";
import { GitHubButton, GoogleButton } from "../components/oauth-buttons";

export default function SignUpPage() {
  return (
    <div className="noise relative flex min-h-screen items-center justify-center">
      <div className="gradient-mesh absolute inset-0" />
      <div className="relative w-full max-w-sm px-6">
        <div className="mb-10 text-center">
          <Heart
            className="mx-auto mb-4 h-8 w-8 text-[var(--pink)]"
            fill="currentColor"
            strokeWidth={0}
          />
          <h1 className="mb-1 text-2xl font-bold">
            <span className="text-[var(--text)]">Join </span>
            <span className="gradient-text">dating.dev</span>
          </h1>
          <p className="font-handwritten text-lg text-[var(--text-dim)]">
            create your profile, find your people
          </p>
        </div>

        <div className="space-y-3">
          <GitHubButton label="Sign up" />
          <GoogleButton label="Sign up" />
        </div>

        <div className="mt-8 text-center">
          <p className="text-xs text-[var(--text-dim)]">
            Already have an account?{" "}
            <a
              href="/signin"
              className="text-[var(--violet-400)] transition-colors hover:text-[var(--pink)]"
            >
              Sign in
            </a>
          </p>
        </div>

        <div className="mt-6 rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] p-4">
          <p className="text-center text-xs leading-relaxed text-[var(--text-dim)]">
            This signs you up for the{" "}
            <span className="text-[var(--violet-400)]">official pool</span>{" "}
            operated by dating.dev. You can also use the{" "}
            <a href="/docs" className="text-[var(--violet-400)] hover:text-[var(--pink)]">
              CLI
            </a>{" "}
            to join any community pool.
          </p>
        </div>
      </div>
    </div>
  );
}
