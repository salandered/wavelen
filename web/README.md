# Wavelen web component

_Web component is mostly written by AI agents._

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
npm run format:check # reports
npm run format       # rewrites
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

## Icons

The collection icons are the SVG sprite at the top of `web/index.html`.

The set that is used for collection creation should be in sync with `internal/icon`.
Adding a collection icon is a `<symbol>` in `web/index.html` and a slug in `internal/icon`.

Some other icons are used for the UI as well.

All icons are from the Lucide toolkit.

### License

Lucide 1.43.0, https://github.com/lucide-icons/lucide

See the included license file `LICENSE-lucide.txt` .
