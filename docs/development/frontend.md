# Frontend development

The PatchBase dashboard is a [SvelteKit](https://kit.svelte.dev/) application in `dashboard/`. It's built with Vite, uses TypeScript, and is embedded into the Go server binary at build time.

## Prerequisites

- **Node.js 20+**
- **pnpm** (package manager)

## Setup

```bash
cd dashboard
pnpm install
```

## Development server

```bash
pnpm dev
```

This starts a Vite dev server (typically on port 5173) with hot module replacement. It proxies API requests to the Go backend on port 5199.

## Building

The frontend is built as part of the Bazel build:

```bash
bazel build //dashboard:dashboard
```

The built output is embedded into the Go binary via `dashboard/embed.go`.

## Formatting

```bash
# Check formatting (CI uses this)
bazel test //dashboard:format_check

# Fix formatting in-place
bazel run //dashboard:format
```

## Linting

```bash
# Check linting (CI uses this)
bazel test //dashboard:lint_check

# Auto-fix linting issues
bazel run //dashboard:lint
```

## Project structure

```
dashboard/
  src/
    routes/          # SvelteKit routes (pages)
      +layout.svelte  # Root layout (auth, navigation)
      +page.svelte    # Dashboard home
      hosts/          # Host list and detail pages
      advisories/     # Advisory browser
      reboots/        # Reboot queue
      settings/       # Server settings
      profile/        # User profile
      login/          # Login page
      setup/          # Initial setup wizard
    lib/              # Shared components and utilities
    app.css           # Global styles
    app.html          # HTML template
  static/             # Static assets
  svelte.config.js    # SvelteKit config
  vite.config.ts      # Vite config
  tsconfig.json       # TypeScript config
```

## Conventions

- Use TypeScript for all new files
- Follow the existing component structure (no CSS frameworks — plain CSS with Svelte scoped styles)
- Use the API client utilities in `lib/` for all backend calls
- WebSocket events are handled in the root layout for live updates