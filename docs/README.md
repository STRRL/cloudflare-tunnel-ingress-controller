# Cloudflare Tunnel Ingress Controller Docs

This directory hosts the documentation site for the Cloudflare Tunnel Ingress Controller, served at [tunnel.strrl.dev](https://tunnel.strrl.dev). The site is built with [Astro](https://astro.build/) and the [Starlight](https://starlight.astro.build/) docs theme.

The controller relies on standard Kubernetes `Ingress` resources (via the `cloudflare-tunnel` ingress class) to publish services through Cloudflare Tunnel—no additional CRDs are required.

## Getting Started

1. Install dependencies:
   ```bash
   pnpm install
   ```
2. Launch the local docs server:
   ```bash
   pnpm dev
   ```
   The site is available at `http://localhost:4321` with hot reload enabled.
3. Build the static site when you are ready to deploy:
   ```bash
   pnpm build
   ```
4. Preview the production build locally:
   ```bash
   pnpm preview
   ```

### Project layout

```
src/
  content/docs/       # Markdown and MDX sources for the documentation
  assets/             # Images and additional media referenced in docs
public/               # Static files copied verbatim to the build output
astro.config.mjs      # Starlight configuration (title, sidebar, social links)
```

Content updates happen inside `src/content/docs/`. Each Markdown file automatically becomes a page; update the sidebar in `astro.config.mjs` when you add new sections.

## Contributing

- Keep new content in English to match the primary audience of the project.
- Prefer short task-focused guides under `src/content/docs/guides/` and longer form references under `src/content/docs/reference/`.
- Run `pnpm build` before submitting changes to ensure there are no build-time regressions.
