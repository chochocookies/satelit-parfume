# packages/

Empty for now on purpose. This becomes a real Turborepo/pnpm workspace
(`ui`, `types`, `config`) once there's a second app or a second package
that actually needs to share code with `apps/web` — see section 96's own
fallback: "If a monorepo becomes unnecessarily complex, a two-repository
structure is acceptable... choose the simplest maintainable architecture."
Right now there's one frontend and nothing to share yet, so adding
workspace tooling here would be structure with nothing in it.
