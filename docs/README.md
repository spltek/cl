# CL — Command Launcher · website

The marketing one‑pager for [`spltek/cl`](https://github.com/spltek/cl), a tiny personal
command launcher: keep a `name → shell command` dictionary in a JSON file and run it from
one interactive picker.

Static site — plain HTML, CSS and vanilla JS. **No build step, no dependencies, no
`package.json`.** Open `index.html` and it runs.

> **This folder is published.** Every push to `main` that touches `docs/` redeploys
> <https://spltek.github.io/cl/>. Edit files here, not a copy kept elsewhere.

```
docs/                          # ← served as-is by GitHub Pages
├── index.html                 # the whole page (one document, all sections)
├── favicon.svg
├── .nojekyll                  # skip Jekyll processing (serves files verbatim)
├── assets/
│   ├── css/main.css           # tokens → components → responsive → guards
│   ├── js/main.js             # terminal replica, placeholder lab, tabs, copies
│   ├── fonts/                 # self-hosted woff2 (Chakra Petch, Inter, JetBrains Mono)
│   │   └── fonts.css
│   └── img/
│       ├── cl-wall.webp|jpg       # hero background (art from the repo's media/cl.png)
│       ├── cl-demo-poster.webp|jpg # demo film poster (YouTube thumbnail, 1280×720)
│       └── og.png                 # 1200×630 social card
└── README.md                  # this file (not linked from the page)
```

## Local development

Any static file server works. `file://` also works, but a server matches production:

```bash
cd docs
python3 -m http.server 8123      # then open http://127.0.0.1:8123/
# or: npx serve .
```

There is nothing to compile. Edit `assets/css/main.css` / `assets/js/main.js` and reload.

## What's interactive

Everything on the page is a working replica of the real TUI, seeded with sample commands:

| Area | Behaviour |
| --- | --- |
| Hero terminal | filter by name (case-insensitive substring), `↑`/`↓` to move, `Enter` to run, `Esc`/`Ctrl-C` to cancel |
| `Ctrl+S` | toggle the command column (names-only ↔ names + commands), display-only, mirrors the real `--show-commands` switch |
| `Ctrl+A` / `Ctrl+E` / `Ctrl+R` / `Ctrl+D` | add, edit, rename, delete — edit/rename/delete end in a `[y/N]` confirm |
| Running | prints the pink `cl-exec: <name>` announcement, exactly like the binary |
| Placeholder lab | live `{{name}}` / `{{name:default}}` resolution against four real templates |
| Install / storage tabs | switch package manager and per-OS config path, every snippet copy-to-clipboard |
| Demo film (`#demo-video`) | poster facade — the YouTube player is injected only when you press play |

State lives in memory only — the demo never writes to `localStorage`, so a reload starts clean.

## Accessibility & performance notes

- Keyboard-first: the picker is focusable, all key handling is scoped to it, and `Ctrl+S`
  is `preventDefault`-ed only while the picker has focus.
- `prefers-reduced-motion: reduce` kills the marquee, scanlines, grain and every reveal
  transition, and forces `[data-reveal]` elements visible.
- Fonts are self-hosted `woff2` with `font-display: swap`; the two above-the-fold faces are
  preloaded. No analytics, no cookies, and no third-party requests on load — the demo film is a
  local poster image until you press play, then it embeds `youtube-nocookie.com`.
- Section reveals use `IntersectionObserver`; no scroll listeners beyond a throttled one
  for the nav progress bar and the footer clock.
- Verified at 1440×900, 820×1180 and 390×844: zero horizontal overflow, no clipped text,
  tap targets ≥ 30 px, small labels keep a legibility floor.

## Deploying

GitHub Pages, straight from this folder — nothing to build:

1. Commit and push `docs/` to `main`.
2. **Settings → Pages → Build and deployment → Source: Deploy from a branch**,
   then pick **`main`** and **`/docs`**. (No Actions workflow needed.)
3. The canonical URL, `og:url` and `og:image` in `index.html` assume
   `https://spltek.github.io/cl/` — update the five absolute URLs in `<head>` if the site
   ever moves to a custom domain.

`.nojekyll` is present so GitHub serves the folder verbatim (no Jekyll pass, no
underscore-prefixed files filtered out).

Optional extras, if you ever want them:

- `docs/404.html` — a themed not-found page served by Pages for unknown paths.
- `docs/robots.txt` / `docs/sitemap.xml`.

## Updating content

- **Commands in the demo** — the `SEED` array near the top of `assets/js/main.js`; the
  placeholder templates sit in `TPLS` further down.
- **Palette / rhythm** — the `:root` block at the top of `assets/css/main.css`
  (`--ink-0..4`, `--cyan`, `--pink`, `--magenta`, `--violet`, `--mint`, `--pad`, `--max`,
  `--nav-h`).
- **Social card** — replace `assets/img/og.png` (1200×630) and keep the `og:image:width` /
  `height` meta in sync.
- **Demo film** — the video id lives in `VIDEO_ID` in `assets/js/main.js` (embed via
  `youtube-nocookie.com`) and the link in the figure caption; the poster is
  `assets/img/cl-demo-poster.jpg|webp`. Swap in a new thumbnail with:
  `curl -o poster.jpg "https://i.ytimg.com/vi/<ID>/maxresdefault.jpg"`.

## License

Site code: MIT. `cl` itself: MIT. Artwork derived from the repo's `media/cl.png`.

Built with [Harny](https://harny.dev) — harny.dev · DeepSeek v4.1 Flash.
