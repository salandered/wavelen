Web component is mostly written by AI agents.

## Formatting

Prettier formats `index.html`, `style.css` and `app.js`.

The pinned version and the config live in `web/package.json`.

Install once:

```sh
cd web
npm ci
```

Run:

```sh
cd web
npm run format:check # reports
npm run format       # rewrites
```

## Linting

ESLint checks `app.js` against its `recommended` set.

```sh
npm run lint
```

The inline script in `index.html` is not covered. ESLint reads no HTML, and the config lists
`app.js` by name.

## Icons

The collection icons are the SVG sprite at the top of `web/index.html`.

The set that is used for collection creation should be in sync with `internal/icon`.
Adding a collection icon is a `<symbol>` in `web/index.html` and a slug in `internal/icon`.

Some other icons are used for the UI as well.

All icons are from the Lucide toolkit.

### License

Lucide 1.43.0, https://github.com/lucide-icons/lucide

See the included license file `LICENSE-lucide.txt` .
