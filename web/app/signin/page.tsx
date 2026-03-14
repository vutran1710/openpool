import { Heart } from "lucide-react";
import { GitHubButton, GoogleButton } from "../components/oauth-buttons";

export default function SignInPage() {
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
            <span className="text-[var(--text)]">Welcome back</span>
          </h1>
          <p className="font-handwritten text-lg text-[var(--text-dim)]">
            pick up where you left off
          </p>
        </div>

        <div className="space-y-3">
          <GitHubButton label="Sign in" />
          <GoogleButton label="Sign in" />
        </div>

        <div className="mt-8 text-center">
          <p className="text-xs text-[var(--text-dim)]">
            Don&apos;t have an account?{" "}
            <a
              href="/signup"
              className="text-[var(--violet-400)] transition-colors hover:text-[var(--pink)]"
            >
              Sign up
            </a>
          </p>
        </div>
      </div>
    </div>
  );
}
