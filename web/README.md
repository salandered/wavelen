# Wavelen web component

_Web component is mostly written using AI agents._

## Formatting

Prettier formats the files under `web/`.

The pinned version and the config live in `web/package.json`.

Install once:

```sh
cd web
npm ci
```

Run:

```sh
cd web
npm run format:check  # reports
npm run format        # rewrites
```

## Linting

ESLint checks `app.js`, `lib.js` and the test file against its `recommended` set.

```sh
npm run lint
```

The inline script in `index.html` is not covered. ESLint reads no HTML, and the config lists
the files by name.

## Testing

`lib.js` contains the page's helpers
`lib.test.js` covers them with Node's own test runner.

```sh
npm test
```

`app.js` is not covered.

## Fonts

Self hosted from `web/fonts/`.

- Inter 4.001 for page text.
- JetBrains Mono 2.304 for hexes.

### License

Inter, https://github.com/rsms/inter

JetBrains Mono 2.304, https://github.com/JetBrains/JetBrainsMono

See `attribution/LICENSE-inter.txt` and `attribution/LICENSE-jetbrains-mono.txt`.

## Palettes

Read-only sets.

- The 100 CSS named colors, from `internal/palette`.
- Open Color, 13 hue families of 10 shades, from `internal/shades`.

### License

Open Color, https://github.com/yeun/open-color

See `attribution/LICENSE-open-color.txt`.

## Icons

The collection icons are the SVG sprite at the top of `web/index.html`.
Those should be in sync with `internal/icon`.

Several harmony icons are drawn for this page (circles with dots representing the degree).

Every other icon is from the Lucide toolkit.

### License

Lucide 1.43.0, https://github.com/lucide-icons/lucide

See the included license file `attribution/LICENSE-lucide.txt` .
