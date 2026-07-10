# PatchBase Dashboard

SvelteKit frontend for PatchBase, served as static files embedded in the Go server binary.

## Stack

- [SvelteKit](https://svelte.dev/docs/kit) with [adapter-static](https://svelte.dev/docs/kit/adapter-static)
- [Vite](https://vite.dev/) for dev server and builds
- [oxlint](https://oxc.rs/docs/guide/usage/lint) for linting
- [oxfmt](https://oxc.rs/docs/guide/usage/format) for formatting
- [pnpm](https://pnpm.io/) as the package manager

## Development

```bash
pnpm install
pnpm run dev
```

The dev server expects the PatchBase API at `http://localhost:5199`. See the main README for instructions on starting the server.

## Building

The dashboard is built via Bazel, which installs dependencies and produces a static bundle that gets embedded into the server binary via `embed.go`:

```bash
bazel build //dashboard:dashboard
```

## Linting and formatting

These are also wired up as Bazel targets used by CI:

```bash
bazel test //dashboard:lint_check    # check
bazel run   //dashboard:lint         # auto-fix

bazel test //dashboard:format_check  # check
bazel run   //dashboard:format       # format in-place
```

## Project layout

```
src/
  routes/     SvelteKit file-based routes (pages)
  lib/        Shared components and utilities
  app.html    Root HTML template
  app.css     Global styles
embed.go      Go embed directive that bundles the build output
```
