import {
	exportFilename,
	labelColor,
	parseHex,
	randomCollectionName,
	randomDigits,
	savedLabel,
} from "./lib.js";

// comes from the spec api.yaml.
const API = "/api/v1";

// "theme" and "bg" stay raw strings. Everything read through readStored is JSON.
const DETAILS_KEY = "details_open";
const ZEN_KEY = "zen";
const DENSE_KEY = "dense";
const PALETTE_DENSE_KEY = "palette_dense";
const PINNED_KEY = "pinned";
const SESSION_KEY = "session";
const CONTROLS_KEY = "controls";
const SELECTION_KEY = "selection";

const $ = (id) => document.getElementById(id);

// ---- browser storage ----
// Every key is optional, see web-wavelen-context.md "Storage".

function readStored(key, fallback) {
	try {
		const raw = localStorage.getItem(key);
		return raw === null ? fallback : JSON.parse(raw);
	} catch {
		return fallback;
	}
}

// A stored JSON object, or {} for anything else in the key. Two callers keep a map under one key.
function readStoredObject(key) {
	const stored = readStored(key, null);
	const usable = stored !== null && typeof stored === "object" && !Array.isArray(stored);
	return usable ? stored : {};
}

function writeStored(key, value) {
	try {
		localStorage.setItem(key, JSON.stringify(value));
	} catch {
		// nothing, the preference doesn't survive the reload
	}
}

// ---- session ----
// The session:
// - token and its expiry from POST /tokens
// - nickname from GET /me
// - id of the collection the saved grid is showing.
// The last two are stored (not re-fetched) => a reload renders without a request.
//
// They are safe stale. The id is checked against the list before it is used.
// The nickname has two uses:
// the caption in the Account panel, the word the delete dialog checks the typed name against.
//
// See web-wavelen-context.md "Storage".

let session = null;

// An expired session is dropped rather than sent, which only saves a request
// that would answer 401. The 401 handling in request() is the actual check.
//
// The collection id is not validated here: pickCollection compares it against the list the account
// has, so anything else in the key falls through to the default.
function loadSession() {
	const stored = readStored(SESSION_KEY, null);
	const usable =
		stored !== null
		&& typeof stored === "object"
		&& typeof stored.token === "string"
		&& typeof stored.expiry === "string"
		&& Date.parse(stored.expiry) > Date.now();

	session = usable ? stored : null;
	if (!usable) {
		writeStored(SESSION_KEY, null);
	}
}

function startSession(token, expiry) {
	session = { token, expiry };
	resetCollections(); // previous account's list and active id belong to nothing now
	writeStored(SESSION_KEY, session);
	renderSession();
}

// It arrives one request after the token, see login(). It is the caption in the Account panel,
// and the delete dialog compares what was typed against it. Only GET /me carries it.
function setSessionNickname(nickname) {
	session = { ...session, nickname };
	writeStored(SESSION_KEY, session);
	renderSession();
}

// A session whose GET /me did not land has a token and no nickname, and stays usable.
// The caption renders without a nickname.
// The delete dialog can't do that (it checks the typed name), so it fetches a nickname.

async function sessionNickname() {
	if (typeof session?.nickname === "string" && session.nickname !== "") {
		return session.nickname;
	}
	const { data } = await call("GET", "/me");
	setSessionNickname(data.user.nickname);
	return data.user.nickname;
}

// Local only.
// Revoking is left to the caller: logout does it, an expiry or a 401 means it is already done.
function endSession() {
	session = null;
	resetCollections();
	writeStored(SESSION_KEY, null);
	renderSession();
}

// ---- requests ----

// Errors come back as {"error": "..."}, but proxy or panic can produce a non-JSON body
// with an error status. So we read the body as text and then try to parse.
//
// The token goes on every request when there is one, public paths included.
// A public path ignores it, so there is one rule here, not a list.
async function call(method, path, body) {
	const options = { method, headers: {} };
	if (session !== null) {
		options.headers.Authorization = `Bearer ${session.token}`;
	}
	if (body !== undefined) {
		options.headers["Content-Type"] = "application/json";
		options.body = JSON.stringify(body);
	}
	const res = await fetch(API + path, options);
	const text = (await res.text()).trim();

	let data = null;
	try {
		data = text === "" ? null : JSON.parse(text);
	} catch {
		// leaves data null, the raw text is what the error below reports
	}

	// There is no refresh, a 401 with a token in hand means that token is finished:
	// expired, or revoked here or from another tab.
	// Going back to logged out.
	if (res.status === 401 && session !== null) {
		endSession();
	}
	if (!res.ok) {
		throw new Error(`${res.status} - ${data?.error ?? text}`);
	}
	return { status: res.status, headers: res.headers, data };
}

// ---- log ----

// The last few things that happened, newest first. In memory only: the entries cover this page
// since it loaded, so a reload starts an empty panel.
const LOG_LIMIT = 10;
const logEntries = [];

// A transient entry is a live counter, so the next entry replaces it instead of landing under it.
// The bulk run uses it to rewrite one line rather than fill the panel with its progress.
function pushLog(message, { failed = false, transient = false } = {}) {
	if (message === "") {
		return; // nothing happened
	}
	if (logEntries[0]?.transient) {
		logEntries.shift();
	}
	logEntries.unshift({ message, failed, transient });
	logEntries.length = Math.min(logEntries.length, LOG_LIMIT);
	renderLog();
}

function renderLog() {
	if (logEntries.length === 0) {
		renderEmpty($("log"), "nothing yet");
		return;
	}
	$("log").replaceChildren(
		...logEntries.map((entry) => {
			const line = document.createElement("p");
			line.textContent = entry.message;
			line.classList.toggle("failed", entry.failed);
			return line;
		}),
	);
}

function setStatus(message, failed = false) {
	pushLog(message, { failed });
}

function setProgress(message) {
	pushLog(message, { transient: true });
}

// /me/... resolves the user from the token server-side. This stops a logged out click from
// sending a request that could only answer 401.
function requireSession() {
	if (session === null) {
		setStatus("log in first", true);
		return false;
	}
	return true;
}

// ---- collections ----
// Every saved-colors path carries a collection id, so the page reads GET /me/collections for it.
// The list is fetched once per session and kept, so a create or a delete edits the local copy
// (not re-fetching). See web-wavelen-context.md "Collections".

let collections = [];
let activeCollection = null;

let collectionsRequest = null;

function ensureCollections() {
	collectionsRequest ??= loadCollections().catch((err) => {
		// cleared, the next click retries instead of replaying the failure
		collectionsRequest = null;
		renderEmpty($("collections"), err.message);
		throw err;
	});
	return collectionsRequest;
}

async function loadCollections() {
	const { data } = await call("GET", "/me/collections");
	collections = data.collections;
	activeCollection = pickCollection(session?.collection);
	renderCollections();
}

// The stored one if the account still has it (or default or oldest or null).
function pickCollection(preferred) {
	const found =
		collections.find((c) => c.id === preferred)
		?? collections.find((c) => c.is_default)
		?? collections[0];
	return found?.id ?? null;
}

function collectionName(id) {
	return collections.find((c) => c.id === id)?.name ?? "";
}

// Awaiting the lookup is what makes the first saved-colors action of a session cost two requests.
async function savedColorsPath() {
	await ensureCollections();
	if (activeCollection === null) {
		throw new Error("this account has no collections");
	}
	return `/me/collections/${activeCollection}/colors`;
}

// The id goes in the session, not under a preference key: it names a row only this account owns,
// so it has to die when the session dies.
function setActiveCollection(id) {
	activeCollection = id;
	session = { ...session, collection: id };
	writeStored(SESSION_KEY, session);
	renderCollections();
}

// Switching collections invalidates the cursor because it belongs to the old collection.
// A non-appended load clears it.
function selectCollection(id) {
	if (id === activeCollection) {
		return;
	}
	setActiveCollection(id);
	setStatus(`showing ${collectionName(id)}`);
	loadSaved();
}

// The next login unhides the section before its request completes.
// Clear the old list so it cannot appear under the new account.
function resetCollections() {
	collections = [];
	activeCollection = null;
	collectionsRequest = null;
	renderCollections();
}

// ---- collection icons ----
// The picker uses the symbols defined in index.html.
// The server keeps the same list in internal/icon and rejects unknown slugs with 400.
const DEF_ICON = "square";

let selectedIcon = DEF_ICON;

// ---- collection accents ----
// Ordering only.
// The server still takes any hex: the ten are this page's set.
const ACCENTS = [
	"gray",
	"red",
	"orange",
	"yellow",
	"green",
	"teal",
	"cyan",
	"blue",
	"violet",
	"pink",
];

// matches [collection.DefIconAccent]
const DEF_ACCENT = "gray";

let selectedAccent = DEF_ACCENT;

// The stored value is always the dark one. --accent-<slug> resolves using the theme
function accentHex(slug) {
	return getComputedStyle(document.documentElement)
		.getPropertyValue(`--accent-${slug}-dark`)
		.trim();
}

// Built once, the reverse of accentHex.
let accentSlugs = new Map();

// 'i-' only: the sprite also holds 'ui-' glyphs the page uses for itself.
function iconNames() {
	return [...document.querySelectorAll("#icon-sprite symbol[id^='i-']")].map((symbol) =>
		symbol.id.replace(/^i-/, ""),
	);
}

// Create SVG elements in the SVG namespace.
// Otherwise the browser treats them as unknown HTML elements and renders nothing.
const SVG_NS = "http://www.w3.org/2000/svg";

function spriteSvg(id, className) {
	const svg = document.createElementNS(SVG_NS, "svg");
	svg.setAttribute("class", className);
	svg.setAttribute("aria-hidden", "true");

	const use = document.createElementNS(SVG_NS, "use");
	use.setAttribute("href", `#${id}`);
	svg.append(use);
	return svg;
}

// a collection icon, by the slug the API stores
function iconSvg(name, className) {
	return spriteSvg(`i-${name}`, className);
}

// Built once: the sprite is static, and only the selected mark and the tint change afterwards.
function renderIconPicker() {
	$("icon-picker").replaceChildren(
		...iconNames().map((name) => {
			const option = document.createElement("button");
			option.type = "button";
			option.className = "icon-option";
			option.dataset.icon = name;
			option.title = name;
			option.setAttribute("role", "radio");
			option.append(iconSvg(name, "icon"));
			option.addEventListener("click", () => {
				selectIcon(name);
				openMenu("icon", false);
			});
			return option;
		}),
	);
	selectIcon(DEF_ICON);
}

function selectIcon(name) {
	selectedIcon = name;
	for (const option of $("icon-picker").children) {
		option.setAttribute("aria-checked", String(option.dataset.icon === name));
	}
	renderIconChoice();
}

function caretSpan() {
	const caret = document.createElement("span");
	caret.className = "caret";
	caret.textContent = "▾";
	return caret;
}

// Apply the current accent to both the trigger and the grid.
// This previews the color a newly created collection would get.
function renderIconChoice() {
	const tint = `var(--accent-${selectedAccent})`;
	$("icon-picker").style.setProperty("--accent-color", tint);
	$("icon-trigger").style.setProperty("--accent-color", tint);

	$("icon-trigger").replaceChildren(iconSvg(selectedIcon, "icon"), caretSpan());
	$("icon-trigger").title = `icon: ${selectedIcon}`;
	$("icon-trigger").setAttribute("aria-label", `collection icon: ${selectedIcon}`);

	const dot = document.createElement("span");
	dot.className = "accent-dot";
	dot.style.setProperty("--accent-color", tint);
	$("accent-trigger").replaceChildren(dot, caretSpan());
	$("accent-trigger").title = `accent: ${selectedAccent}`;
	$("accent-trigger").setAttribute("aria-label", `collection accent: ${selectedAccent}`);
}

// Built once from ACCENTS, the same shape renderIconPicker builds from the sprite.
function renderAccentPicker() {
	accentSlugs = new Map(ACCENTS.map((slug) => [accentHex(slug), slug]));
	$("accent-picker").replaceChildren(
		...ACCENTS.map((slug) => {
			const option = document.createElement("button");
			option.type = "button";
			option.className = "accent-option";
			option.dataset.accent = slug;
			option.title = slug;
			option.setAttribute("role", "radio");
			const dot = document.createElement("span");
			dot.className = "accent-dot";
			dot.style.setProperty("--accent-color", `var(--accent-${slug})`);
			option.append(dot);
			option.addEventListener("click", () => {
				selectAccent(slug);
				openMenu("accent", false);
			});
			return option;
		}),
	);
	selectAccent(DEF_ACCENT);
}

function selectAccent(slug) {
	selectedAccent = slug;
	for (const option of $("accent-picker").children) {
		option.setAttribute("aria-checked", String(option.dataset.accent === slug));
	}
	renderIconChoice();
}

// The icon grid and the accent grid behave the same way
const MENUS = ["icon", "accent"];

// aria-expanded is the source of truth for the menu state.
// [hidden] hides the grid when it is closed, while CSS controls its display when open.
function openMenu(name, open) {
	$(`${name}-trigger`).setAttribute("aria-expanded", String(open));
	$(`${name}-picker`).hidden = !open;
}

function menuOpen(name) {
	return $(`${name}-trigger`).getAttribute("aria-expanded") === "true";
}

// One tab per collection, and the tab only selects.
// The trash is disabled while the default is the active one (server would refuse).
function renderCollections() {
	const active = collections.find((c) => c.id === activeCollection);
	$("collection-delete").disabled = active === undefined || active.is_default;

	if (collections.length === 0) {
		renderEmpty($("collections"), "none");
		return;
	}
	$("collections").replaceChildren(
		...collections.map((col) => {
			const row = document.createElement("div");
			row.className = "collection";
			row.classList.toggle("active", col.id === activeCollection);

			const name = document.createElement("button");
			name.type = "button";
			name.className = "collection-name";
			const showing = col.is_default ? "showing this one, the default" : "showing this one";
			name.title = col.id === activeCollection ? showing : `show ${col.name}`;
			name.addEventListener("click", () => selectCollection(col.id));

			const glyph = iconSvg(col.icon, "icon collection-icon");
			// token for one of the ten, so a theme flip re-resolves it with nothing re-rendered
			const slug = accentSlugs.get(col.accent);
			glyph.style.setProperty(
				"--accent-color",
				slug === undefined ? col.accent : `var(--accent-${slug})`,
			);
			const text = document.createElement("span");
			text.className = "name";
			text.textContent = col.name;
			name.append(glyph, text);
			row.append(name);
			return row;
		}),
	);
}

// Confirm before deleting the active collection.
// The active collection is the one currently shown in the grid.
async function deleteCollection() {
	if (!requireSession()) {
		return;
	}
	await ensureCollections();
	const col = collections.find((c) => c.id === activeCollection);
	if (col === undefined || col.is_default) {
		return; // button is disabled in both cases, see renderCollections
	}
	if (!confirm(`delete ${col.name} and every color in it?`)) {
		return;
	}
	try {
		await call("DELETE", `/me/collections/${col.id}`);
		collections = collections.filter((c) => c.id !== col.id);
		setStatus(`deleted ${col.name}`);
		// grid is showing a collection that no longer exists
		setActiveCollection(pickCollection(null));
		await loadSaved();
	} catch (err) {
		setStatus(err.message, true);
	}
}

// The collection itself stays here.
async function emptyCollection() {
	if (!requireSession()) {
		return;
	}

	// resolved before the question
	let path;
	try {
		path = await savedColorsPath();
	} catch (err) {
		setStatus(err.message, true);
		return;
	}

	const name = collectionName(activeCollection);
	if (!confirm(`delete every color in ${name}?`)) {
		return;
	}
	try {
		await call("DELETE", path);
		setStatus(`emptied ${name}`);
		await loadSaved();
	} catch (err) {
		setStatus(err.message, true);
	}
}

// The list is ordered oldest first, which is where a new row belongs.
async function createCollection(name, icon, accent) {
	await ensureCollections();
	const { data } = await call("POST", "/me/collections", { name, icon, accent });
	collections.push(data.collection);
	setStatus(`${data.collection.name} created`);
	selectCollection(data.collection.id);
}

// ---- swatches ----

// The selection contains a hex value and a label.
// It marks matching swatches and fills the detail panel.
// Clear it when the hex field no longer matches.
let selectedHex = null;
let selectedLabel = "";

// What the page opens on: the title's first tint, see the h1[data-tint="1"] rule. Not a palette
// row, so no swatch carries the mark until the first click. The label names the source, the way
// "picked" and "random" do.
const DEF_SELECTION = { hex: "#bf15a3", name: "title" };

// The selection survives a reload, so the page comes back on the color that was being looked at
// rather than on the title's tint. A hex that no longer parses falls through to DEF_SELECTION
// (similar to a stored control outside CONTROL_VALUES).
//
// A dropped selection is stored as null and opens on DEF_SELECTION too: restoring the empty state
// would put the three blank panels back, DEF_SELECTION exists to avoid that.
function storedSelection() {
	const stored = readStoredObject(SELECTION_KEY);
	const hex = typeof stored.hex === "string" ? parseHex(stored.hex) : null;
	if (hex === null || typeof stored.name !== "string") {
		return DEF_SELECTION;
	}
	return { hex, name: stored.name };
}

function markSelected() {
	for (const el of document.querySelectorAll(".swatch")) {
		el.classList.toggle("selected", el.dataset.hex === selectedHex);
	}
}

function swatchCell(hex, label) {
	const cell = document.createElement("div");
	cell.className = "cell";

	const el = document.createElement("button");
	el.type = "button";
	el.className = "swatch";
	el.dataset.hex = hex;
	el.style.background = hex;
	el.title = `use ${hex}`;
	el.setAttribute("aria-label", `use ${hex}`); // it holds no text of its own
	el.addEventListener("click", () => selectColor(hex, label));

	const text = document.createElement("div");
	text.className = "swatch-text";
	text.style.color = labelColor(hex);

	const code = document.createElement("button");
	code.type = "button";
	code.className = "swatch-hex";
	code.textContent = hex;
	code.title = "copy";
	code.addEventListener("click", () => copyHex(hex));

	const caption = document.createElement("span");
	caption.className = "label";
	caption.textContent = label;

	// label on top, hex under it
	text.append(caption, code);
	cell.append(el, text);
	return cell;
}

// The Selected panel's picker mirrors the selection: it adjusts the color it shows
function selectColor(hex, label) {
	$("pick").value = hex;
	commitSelection(hex, label);
}

// Every producer comes through here, so the inputs and the panels beside them cannot disagree.
function commitSelection(hex, label) {
	selectedHex = hex;
	selectedLabel = label;
	selectionChanged();
}

// Both detail panels depend on the same selection, so update them together.
function selectionChanged() {
	markSelected();
	renderDetail();
	// panel controls
	for (const id of ["pick", "selected-add", "selected-fullscreen"]) {
		$(id).disabled = selectedHex === null;
	}

	// this is the one place the reload has to read back (every producer ends here)
	const stored = selectedHex === null ? null : { hex: selectedHex, name: selectedLabel };
	writeStored(SELECTION_KEY, stored);

	// every pinned strip follows the selection or empties with it (one built for the previous
	// color would be wrong)
	showStrips();
}

// Saved cell carries a delete button as well.
// The grid and hover styles are applied to the cell.
function savedSwatch(hex, label) {
	const cell = swatchCell(hex, label);

	const remove = document.createElement("button");
	remove.type = "button";
	remove.className = "remove";
	remove.textContent = "×";
	remove.style.color = labelColor(hex); // it sits on the color, like the hex
	remove.title = `delete ${hex}`;
	remove.setAttribute("aria-label", `delete ${hex}`);
	remove.addEventListener("click", () => deleteColor(hex, cell, remove));

	cell.append(remove);
	return cell;
}

// Palette cell has an add button (same as saved cell has delete).
function paletteSwatch(hex, label) {
	const cell = swatchCell(hex, label);
	const add = addButton(() => hex);
	labelAddButton(add, hex);
	cell.append(add);
	return cell;
}

function addButton(readHex) {
	const add = document.createElement("button");
	add.type = "button";
	add.className = "add";
	add.append(spriteSvg("ui-circle-plus", "icon"));
	add.addEventListener("click", () => addColor(readHex(), add));
	return add;
}

function labelAddButton(add, hex) {
	add.style.color = labelColor(hex); // it sits on the color, like the hex
	add.title = `add ${hex}`;
	add.setAttribute("aria-label", `add ${hex}`);
}

// The path takes the six digits: api.yaml rejects a '#' (even if escaped).
//
// The cell is dropped rather than the list reloaded (a reload would drop every appended page).
// The cursor survives a delete: it is a value compared against, not a reference to a row.
async function deleteColor(hex, cell, button) {
	if (!requireSession()) {
		return;
	}
	button.disabled = true; // a second click while the first is out would answer 404
	try {
		await call("DELETE", `${await savedColorsPath()}/${hex.slice(1)}`);
		cell.remove();
		if ($("saved").childElementCount === 0) {
			renderEmpty($("saved"), "nothing saved");
		}
		setStatus(`deleted ${hex}`);
	} catch (err) {
		button.disabled = false;
		setStatus(err.message, true);
	}
}

// One color into the active collection
async function addColor(hex, button) {
	if (!requireSession()) {
		return;
	}
	button.disabled = true; // second click would answer nothing new
	try {
		const { status, data } = await call("POST", await savedColorsPath(), { hex });
		setStatus(status === 201 ? `added ${data.hex}` : `${data.hex} was already saved`);
		await loadSaved();
	} catch (err) {
		setStatus(err.message, true);
	} finally {
		button.disabled = false;
	}
}

function renderEmpty(container, message) {
	const p = document.createElement("p");
	p.className = "empty";
	p.textContent = message;
	container.replaceChildren(p);
}

// Every full screen path comes through here, see web-wavelen-context.md "Full screen".
// An iframe without allowfullscreen or a browser policy can refuse.
async function toggleFullscreen(el) {
	try {
		if (document.fullscreenElement === null) {
			await el.requestFullscreen();
		} else {
			await document.exitFullscreen();
		}
	} catch (err) {
		setStatus(`full screen refused - ${err.message}`, true);
	}
}

// Use the button that was pressed.
// The delegated capture listener has already added the "pressed" class, so remove it before toggling.
function toggleFullscreenFrom(button, el) {
	if (document.fullscreenElement !== null) {
		button.classList.remove("pressed");
	}
	toggleFullscreen(el);
}

// navigator.clipboard exists only in a secure context, so over plain http the property is missing
// and reading through it throws. In an async function that is a rejection.
// The dip on a press is on every button, so a copy needs none of its own.
// The log line separates a copy that failed from one that went through.
async function copyHex(hex) {
	try {
		await navigator.clipboard.writeText(hex);
		setStatus(`copied ${hex}`);
	} catch (err) {
		setStatus(`could not copy - ${err.message}`, true);
	}
}

// Clipboard access may fail due to permissions or an insecure context.
// Report invalid clipboard contents instead of putting them in the field.
// Otherwise the field would clear the current selection without giving useful feedback.
async function pasteHex() {
	let text;
	try {
		text = await navigator.clipboard.readText();
	} catch (err) {
		setStatus(`could not read the clipboard - ${err.message}`, true);
		return;
	}
	const hex = parseHex(text);
	if (hex === null) {
		setStatus("the clipboard is not a hex color", true);
		return;
	}
	$("hex").value = hex;
}

// One panel for both grids, since the selection they share is one hex.
function renderDetail() {
	if (selectedHex === null) {
		$("detail").replaceChildren();
		return;
	}
	const hex = selectedHex; // captured, so a later selection does not rewrite these handlers

	// siblings, not one inside the other, so neither click has to be kept from reaching the other
	const stack = document.createElement("div");
	stack.className = "detail-stack";

	const block = document.createElement("div");
	block.className = "detail-color";
	block.style.background = hex;
	block.addEventListener("click", () => {
		if (document.fullscreenElement === block) {
			toggleFullscreen(block);
		}
	});

	const code = document.createElement("button");
	code.type = "button";
	code.className = "detail-hex";
	code.style.color = labelColor(hex);
	code.textContent = hex;
	code.title = "copy";
	code.addEventListener("click", () => copyHex(hex));

	const add = addButton(() => hex);
	labelAddButton(add, hex);

	stack.append(block, code, add);

	const caption = document.createElement("p");
	caption.className = "detail-label";
	caption.textContent = selectedLabel;

	$("detail").replaceChildren(stack, caption);
}

// ---- color pickers ----
// Several <input type="color"> elements. One of them
// in the Selected panel touches the selection: it nudges the color that panel shows. 
//
// A color picker emits many values during a drag.
// Harmony changes trigger requests, so wait until the drag pauses before committing the selection.
const PICK_QUIET = 200;

let pickTimer = null;

function initPicker() {
	// Browsers differ in how they emit input and change events during a drag.
	// Treat both events the same and use one timer to collapse the gesture into a single commit.
	for (const type of ["input", "change"]) {
		$("pick").addEventListener(type, () => {
			clearTimeout(pickTimer);
			// no write back into the input: it produced the value and its dialog is still open
			pickTimer = setTimeout(() => commitSelection($("pick").value, "picked"), PICK_QUIET);
		});
	}

	for (const type of ["input", "change"]) {
		$("add-pick").addEventListener(type, () => {
			$("hex").value = $("add-pick").value;
		});
	}
}

function renderScratch() {
	$("scratch-hex").textContent = $("scratch").value;
}

// the label follows every step of a drag: nothing here costs a request
function initScratch() {
	for (const type of ["input", "change"]) {
		$("scratch").addEventListener(type, renderScratch);
	}
	$("scratch-hex").addEventListener("click", () => copyHex($("scratch").value));
	renderScratch();
}

// ---- derived strips ----

// The API derives these names from one color, in its own order out of color.HarmonyNames. One
// endpoint answers for all of them in one shape, so one loader draws any of them. Each name is the
// suffix of its pin's id and of the div its strip lands in.
const DERIVED = ["complement", "split-complement", "triad", "analogous", "square", "ramp", "tones"];

// Rotations include the selected color as their first band.
// Scales contain seven axis values and do not include the selected color.
// Adding it would make the scale appear out of order.
const LEADING = new Set(["complement", "split-complement", "triad", "analogous", "square"]);

// A scale sweeps an axis, and its name leaves that out. The pin and the strip's own label both
// carry it as a title.
const AXIS = {
	ramp: "lightness, dark to light",
	tones: "chroma, the gray of this lightness to the full color",
};

// The page opens on one rotation and both scales, the set it showed before the pins.
const DEF_PINNED = ["complement", "ramp", "tones"];

function harmonySpace() {
	return readControl("space");
}

// Cache each strip as { hex, space, bands }.
// Reuse it when the selection and color space have not changed.
// This also prevents stale bands from appearing or being added.
const stripColors = {};

// Same guard as loadSaved, one number for the whole section: the pins stay live while a request is
// out, so an earlier response landing later must not replace a fresher strip.
let stripGeneration = 0;

// A closed section sends no request. Opening it loads what it missed.
let stripsStale = false;

// The pinned names, in DERIVED order: the strips read top to bottom the way the pins read
// left to right, whatever order they were pinned in.
let pinned = [];

function applyPinned(names) {
	pinned = DERIVED.filter((name) => names.includes(name));
	for (const name of DERIVED) {
		$(`pin-${name}`).setAttribute("aria-pressed", String(pinned.includes(name)));
	}
	writeStored(PINNED_KEY, pinned);
	renderStrips();
}

function togglePin(name) {
	applyPinned(pinned.includes(name) ? pinned.filter((other) => other !== name) : [...pinned, name]);
}

// Build one block for each pinned harmony.
// The same structure works for both rotations and scales.
function renderStrips() {
	if (pinned.length === 0) {
		renderEmpty($("strips"), "nothing pinned");
		return;
	}
	$("strips").replaceChildren(...pinned.map(stripBlock));
	showStrips();
}

function stripBlock(name) {
	const block = document.createElement("div");
	// a rotation is two to four bands and shares a row, a scale is seven and takes one
	block.className = LEADING.has(name) ? "strip-block rotation" : "strip-block scale";
	block.dataset.strip = name; // F key is over this, see fullscreenUnderPointer

	const toolbar = document.createElement("div");
	toolbar.className = "toolbar";

	const named = document.createElement("div");
	named.className = "row";
	const label = document.createElement("span");
	label.className = "strip-name";
	// the same glyph the pin carries, inside the label: the two are one name, not a row of two
	if (LEADING.has(name)) {
		label.append(spriteSvg(`ui-h-${name}`, "icon strip-glyph"));
	}
	label.append(name);
	if (AXIS[name] !== undefined) {
		label.title = AXIS[name];
	}
	named.append(label);

	const controls = document.createElement("div");
	controls.className = "row";
	controls.append(
		iconButton("ui-circle-plus", `add the ${name} to this collection`, (event) =>
			addStrip(name, event.currentTarget),
		),
		iconButton("ui-monitor", `the ${name} below, full screen`, () => showStripFullscreen(name)),
	);

	toolbar.append(named, controls);

	const strip = document.createElement("div");
	strip.id = `strip-${name}`;

	block.append(toolbar, strip);
	return block;
}

function iconButton(symbol, label, onClick) {
	const button = document.createElement("button");
	button.type = "button";
	button.className = "icon-button";
	button.title = label;
	button.setAttribute("aria-label", label);
	button.append(spriteSvg(symbol, "icon"));
	button.addEventListener("click", onClick);
	return button;
}

// stripColors keeps the last bands drawn for each pin. A pin is redrawn from them without a
// request when both selection and wheel hasn't changed.
function showStrips() {
	if (!$("harmony-section").open) {
		stripsStale = true;
		return;
	}
	stripsStale = false;

	const generation = ++stripGeneration;
	const space = harmonySpace();
	for (const name of pinned) {
		if (selectedHex === null) {
			delete stripColors[name];
			drawStrip(name, null);
		} else if (stripColors[name]?.hex === selectedHex && stripColors[name].space === space) {
			drawStrip(name, stripColors[name].bands);
		} else {
			loadStrip(name, selectedHex, space, generation);
		}
	}
}

async function loadStrip(name, hex, space, generation) {
	try {
		// A rotation names its wheel even when it is the server's default: the bare URL is cached
		// immutable and has already meant two wheels. A scale holds the hue and keeps that URL.
		const wheel = LEADING.has(name) ? `?space=${space}` : "";
		// bare six digits in the path, same rule as the delete above
		const { data } = await call("GET", `/colors/${hex.slice(1)}/${name}${wheel}`);
		if (generation !== stripGeneration) {
			return; // a newer selection is current
		}
		// the answer is the normalized hex asked about, the name, and the colors it derives
		const bands = LEADING.has(name) ? [data.hex, ...data.colors] : data.colors;
		stripColors[name] = { hex, space, bands };
		drawStrip(name, bands);
	} catch (err) {
		if (generation !== stripGeneration) {
			return;
		}
		delete stripColors[name];
		drawStrip(name, null);
		setStatus(err.message, true);
	}
}

// A band selects (same as swatch), so a step can be picked up and worked on.
// A strip already on the page is refilled (not rebuilt), see fillStrip.
function drawStrip(name, bands) {
	const target = $(`strip-${name}`);
	if (target === null) {
		return; // unpinned while its request was out
	}
	if (bands === null) {
		target.replaceChildren();
		return;
	}
	const onBand = (band) => selectColor(band, name);
	const strip = target.querySelector(".harmony");
	if (strip === null) {
		target.replaceChildren(buildStrip(bands, onBand));
		return;
	}
	fillStrip(strip, bands, onBand);
}

function showSelectedFullscreen() {
	const color = document.querySelector(".detail-color");
	if (color === null) {
		setStatus("nothing to show", true);
		return;
	}
	toggleFullscreen(color);
}

function showStripFullscreen(name) {
	const strip = $(`strip-${name}`)?.querySelector(".harmony") ?? null;
	if (strip === null) {
		setStatus("nothing to show", true);
		return;
	}
	toggleFullscreen(strip);
}

// Add exactly the bands currently shown in the strip, including the selected color.
// 200 means the color already existed; 201 means it was newly created.
// => 'created' counts only new colors.
async function addStrip(name, button) {
	if (!requireSession()) {
		return;
	}
	const bands = stripColors[name]?.bands ?? [];
	if (bands.length === 0) {
		setStatus("nothing to add", true);
		return;
	}

	let path;
	try {
		path = await savedColorsPath();
	} catch (err) {
		setStatus(err.message, true);
		return;
	}

	let created = 0;
	let failed = 0;
	let firstError = null;

	button.disabled = true;
	try {
		for (const [done, hex] of bands.entries()) {
			try {
				const { status } = await call("POST", path, { hex });
				if (status === 201) {
					created++;
				}
			} catch (err) {
				failed++;
				firstError ??= err.message;
			}
			setProgress(`adding the ${name}... ${done + 1}/${bands.length}`);
		}
	} finally {
		button.disabled = false;
	}

	const summary = `added ${created} of ${bands.length} to ${collectionName(activeCollection)}`;
	setStatus(failed === 0 ? summary : `${summary}, ${failed} failed - ${firstError}`, failed > 0);
	await loadSaved();
}

// Built from the swatches on screen, so what goes up is what the grid shows.
function showSavedFullscreen() {
	const hexes = [...document.querySelectorAll("#saved .swatch")].map((el) => el.dataset.hex);
	if (hexes.length === 0) {
		setStatus("nothing to show", true);
		return;
	}
	showOffstage(buildStrip(hexes));
}

// Show the palette as a grid, using a clone of the current grid.
// Full-screen mode is always dense.
// See web-wavelen-context.md "Full screen".
function showPaletteFullscreen() {
	const grid = $("palette");
	if (grid.querySelector(".swatch") === null) {
		setStatus("nothing to show", true);
		return;
	}
	const copy = grid.cloneNode(true);
	copy.classList.add("dense");
	copy.removeAttribute("id"); // two of an id, and $("palette") could answer with this one

	// Inert, like the listeners the clone dropped. Dropping data-hex is deliberate: it keeps a
	// later markSelected off these cells.
	for (const el of copy.querySelectorAll(".swatch")) {
		el.classList.remove("selected");
		el.removeAttribute("title");
		delete el.dataset.hex;
	}
	copy.addEventListener("click", (event) => {
		const swatch = event.target.closest(".swatch");
		if (swatch !== null) {
			toggleFullscreenFrom(swatch, copy);
		}
	});
	showOffstage(copy);
}

// An element that is on the page only to be full screen: off stage in <body> while it is up, and
// gone when it comes down.
async function showOffstage(el) {
	el.classList.add("offstage");
	document.body.append(el);

	// leaving full screen - by Esc or by a click inside
	document.addEventListener("fullscreenchange", function onLeave() {
		if (document.fullscreenElement === null) {
			document.removeEventListener("fullscreenchange", onLeave);
			el.remove();
		}
	});
	try {
		await el.requestFullscreen();
	} catch (err) {
		el.remove();
		setStatus(`full screen refused - ${err.message}`, true);
	}
}

// Without onBand a band click toggles full screen.
// A caller that passes one takes the click instead, see drawStrip.
function buildStrip(hexes, onBand) {
	const strip = document.createElement("div");
	strip.className = "harmony";
	fillStrip(strip, hexes, onBand);
	return strip;
}

/*
The colors of a strip that is already up, onto the bands it already has. A band clicked to select
reloads every pinned strip, and a rebuilt one takes that band off the page while its press dip is
still playing. So the band count is matched and the rest is written over what is there.

A strip keeps its count across a selection (a triad stays three). So the loops below are the
edges: a first fill, and the saved grid's strip, which is a page of swatches and can be any
length.
*/
function fillStrip(strip, hexes, onBand) {
	while (strip.children.length > hexes.length) {
		strip.lastElementChild.remove();
	}
	while (strip.children.length < hexes.length) {
		strip.append(buildBand(strip, onBand));
	}

	// every band opens the same strip, so they share one label rather than each naming its color
	const fullscreenLabel = onBand === undefined ? `show ${hexes.join(" ")} full screen` : null;
	for (const [index, hex] of hexes.entries()) {
		const band = strip.children[index];
		band.dataset.hex = hex;

		const block = band.querySelector(".band-color");
		block.style.background = hex;
		block.setAttribute("aria-label", fullscreenLabel ?? hex);

		const code = band.querySelector(".band-hex");
		code.style.color = labelColor(hex);
		code.textContent = hex;

		const add = band.querySelector(".add");
		if (add !== null) {
			labelAddButton(add, hex);
		}
	}
}

// The listeners read the band's own dataset rather than closing over a hex: the element outlives
// the color it was built with, see fillStrip.
function buildBand(strip, onBand) {
	const band = document.createElement("div");
	band.className = "band";

	const block = document.createElement("button");
	block.type = "button";
	block.className = "band-color";
	if (onBand === undefined) {
		// no title: built off stage, so there is no page to read a tip on
		block.addEventListener("click", () => toggleFullscreenFrom(block, strip));
	} else {
		block.addEventListener("click", () => {
			// full screen is the strip itself, so a band in it only leaves. The exit is explicit
			// here because a refill keeps the strip on the page, see fillStrip.
			if (document.fullscreenElement === strip) {
				toggleFullscreenFrom(block, strip);
				return;
			}
			onBand(band.dataset.hex);
		});
	}

	const code = document.createElement("button");
	code.type = "button";
	code.className = "band-hex";
	code.title = "copy";
	code.addEventListener("click", () => copyHex(band.dataset.hex));

	band.append(block, code);

	if (onBand !== undefined) {
		band.append(addButton(() => band.dataset.hex));
	}
	return band;
}

// markSelected, so a swatch keeps its mark across a re-render.
function renderSwatches(container, swatches) {
	container.replaceChildren(...swatches);
	markSelected();
}

// Keeps the pages already shown. These swatches were built after the last markSelected and carry
// no mark of their own.
function appendSwatches(container, swatches) {
	container.append(...swatches);
	markSelected();
}

// ---- data ----

// Sorting by hex is lexicographic, so it groups by the red channel rather than perceptually.
async function loadPalette() {
	const params = new URLSearchParams({
		sort: readControl("palette-sort"),
		order: readControl("palette-order"),
	});
	try {
		const { data } = await call("GET", `/colors?${params}`);
		renderSwatches(
			$("palette"),
			data.colors.map((c) => paletteSwatch(c.hex, c.name)),
		);
	} catch (err) {
		renderEmpty($("palette"), err.message);
	}
}

// Cursor for the next saved-colors page, or null at the end.
// It is tied to the sort and order used to create it.
// It is not tied to a user, so clear it explicitly on logout.
let nextCursor = null;

// Multiple loadSaved calls can overlap.
// Each call gets a generation number and updates the grid only if it is still current.
// This prevents an older response from overwriting a newer one.
let savedGeneration = 0;

// The button is the only way to page, so hiding it is the end-of-list signal.
function setNextCursor(cursor) {
	nextCursor = cursor;
	$("load-more").hidden = cursor === null;
}

function savedQuery(cursor) {
	const params = new URLSearchParams({
		sort: readControl("sort"),
		order: readControl("order"),
		limit: readControl("limit"),
	});
	if (cursor !== null) {
		params.set("cursor", cursor);
	}
	return params;
}

async function loadSaved({ append = false } = {}) {
	if (!requireSession()) {
		return;
	}
	const cursor = append ? nextCursor : null;
	const generation = ++savedGeneration;

	// Keep the current cursor until the request completes.
	// Without disabling the button, a second click could request the same page twice.
	$("load-more").disabled = true;
	try {
		const { data } = await call("GET", `${await savedColorsPath()}?${savedQuery(cursor)}`);
		if (generation !== savedGeneration) {
			return; // a newer load is current
		}
		const swatches = data.colors.map((c) => savedSwatch(c.hex, savedLabel(new Date(c.created_at))));

		if (append) {
			appendSwatches($("saved"), swatches);
		} else if (swatches.length === 0) {
			renderEmpty($("saved"), "nothing saved");
		} else {
			renderSwatches($("saved"), swatches);
		}
		setNextCursor(data.metadata.next_cursor ?? null);
	} catch (err) {
		if (generation !== savedGeneration) {
			return;
		}
		if (!append) {
			renderEmpty($("saved"), ""); // an appended page failing leaves the pages already shown
		}
		setNextCursor(null);
		setStatus(err.message, true);
	} finally {
		// only the newest request re-enables it, so it stays disabled while another one is out
		if (generation === savedGeneration) {
			$("load-more").disabled = false;
		}
	}
}

// ---- random values ----
// ---- bulk add ----
// API has no bulk endpoint, ordinary POSTs

const BULK_COUNT = 5;

async function addRandomColors() {
	if (!requireSession()) {
		return;
	}

	// Resolved once, before any request goes out.
	let path;
	try {
		path = await savedColorsPath();
	} catch (err) {
		setStatus(err.message, true);
		return;
	}

	// a set, so the run is BULK_COUNT distinct colors rather than that many draws
	const queue = new Set();
	while (queue.size < BULK_COUNT) {
		queue.add("#" + randomDigits());
	}

	let done = 0;
	let created = 0;
	let failed = 0;
	let firstError = null;

	// one failure is counted, not thrown: the run reports what it managed either way
	const add = async (hex) => {
		try {
			const { status } = await call("POST", path, { hex });
			if (status === 201) {
				created++;
			}
		} catch (err) {
			failed++;
			firstError ??= err.message;
		}
		done++;
		setProgress(`adding colors... ${done}/${BULK_COUNT}`);
	};

	$("bulk-add").disabled = true;
	try {
		await Promise.all([...queue].map(add));
	} finally {
		$("bulk-add").disabled = false;
	}

	// created counts 201s only, so the rest were colors this user already had
	const summary = `added ${created} of ${BULK_COUNT}`;
	setStatus(failed === 0 ? summary : `${summary}, ${failed} failed - ${firstError}`, failed > 0);
	await loadSaved();
}

// ---- account ----

// Login and signup use the same fields, only one form is shown at a time.
// The selected tab is not persisted because the dialog only opens while logged out.
const ACCOUNT_TABS = ["login", "signup"];

// 'aria-selected' is the source of truth for the active tab.
// When the dialog is open, focus moves into the selected form.
function selectAccountTab(name) {
	for (const tab of ACCOUNT_TABS) {
		const selected = tab === name;
		$(`tab-${tab}`).setAttribute("aria-selected", String(selected));
		$(`${tab}-form`).hidden = !selected;
	}
	if ($("account").open) {
		$(`${name}-form`).querySelector("input").focus();
	}
}

function openAccount() {
	showAccountError("");
	$("account").showModal();
	selectAccountTab("login");
}

// The log panel is behind the backdrop while the dialog is up, so a failure is written here as
// well. Cleared on open and on success, since the line is only there while it has text.
function showAccountError(message) {
	$("account-error").textContent = message;
}

// Only Collections and Saved colors need a token. The palette and every harmony are public, so a
// logged out visitor keeps a working page.
function renderSession() {
	$("login-open").hidden = session !== null;
	$("who").hidden = session === null;
	$("saved-section").hidden = session === null;
	// logout must not leave the menu standing over the next login's header
	openAccountMenu(false);

	if (session === null) {
		// the cursor was minted for the session that just ended and carries no user of its own,
		// so a later load-more would append the previous account's next page
		setNextCursor(null);
		// the grid still holds that account's swatches, and the next login unhides this section
		// before its own request lands
		renderEmpty($("saved"), "nothing saved");
		return;
	}
	// span, not button: the chip truncates its name
	// empty while GET /me is in flight, and after it failed
	$("who").querySelector(".name").textContent = session.nickname || "logged in";
}

// aria-expanded is the source of truth, the same as the icon menu.
function openAccountMenu(open) {
	$("who").setAttribute("aria-expanded", String(open));
	$("account-drop").hidden = !open;
}

function accountMenuOpen() {
	return $("who").getAttribute("aria-expanded") === "true";
}

// Login makes two requests: create the token, then fetch the account.
// The session must exist before GET /me can run.
// The account panel therefore renders without a nickname until that request completes.
async function login(nickname, password) {
	const { data } = await call("POST", "/tokens", { nickname, password });
	startSession(data.token, data.expiry);
	showAccountError("");
	$("account").close();

	const { data: me } = await call("GET", "/me");
	setSessionNickname(me.user.nickname);
	setStatus(`logged in as ${me.user.nickname}`);
	await loadSaved();
}

// ---- preferences ----

function currentTheme() {
	return (
		document.documentElement.dataset.theme
		?? (matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light")
	);
}

// The glyph is the theme a click switches to, as the text label was. sun and moon are two of the
// collection icons, so the sprite already holds them.
function renderThemeButton(theme) {
	const next = theme === "dark" ? "light" : "dark";
	$("theme").replaceChildren(iconSvg(next === "light" ? "sun" : "moon", "icon"));
	$("theme").title = `${next} theme (L)`;
	$("theme").setAttribute("aria-label", `switch to ${next} theme`);
}

function applyTheme(theme) {
	document.documentElement.dataset.theme = theme;
	renderThemeButton(theme);
	try {
		// raw, not writeStored: the <head> script reads this one back without parsing it
		localStorage.setItem("theme", theme);
	} catch {
		// the preference doesn't survive a reload
	}
}

// EXPERIMENTAL. The page fill, off by default. Mirrors [data-bg] in style.css.
const BG_VALUES = ["off", "soft", "deep"];

function currentBg() {
	return document.documentElement.dataset.bg ?? "off";
}

function nextBg(value) {
	return BG_VALUES[(BG_VALUES.indexOf(value) + 1) % BG_VALUES.length];
}

function applyBg(value) {
	if (value === "off") {
		// the default is the absence of the attribute, so --bg stays the Canvas keyword
		delete document.documentElement.dataset.bg;
	} else {
		document.documentElement.dataset.bg = value;
	}
	$("bg").dataset.value = value;
	$("bg").textContent = value;
	$("bg").title = `switch the page fill to ${nextBg(value)} (B)`;
	try {
		// raw, not writeStored: the <head> script reads this one back without parsing it
		localStorage.setItem("bg", value);
	} catch {
		// the preference doesn't survive a reload
	}
}

function applyZen(on) {
	document.documentElement.classList.toggle("zen", on);
	// the glyph names what a click does, as the two words it replaced did
	$("zen").replaceChildren(spriteSvg(on ? "ui-eye" : "ui-eye-off", "icon"));
	$("zen").title = "zen mode (Z)";
	$("zen").setAttribute("aria-pressed", String(on));
	$("zen").setAttribute("aria-label", on ? "leave zen mode" : "zen mode");
	writeStored(ZEN_KEY, on);
}

function zenOn() {
	return document.documentElement.classList.contains("zen");
}

// Both grids take the shape from a toggle of their own, and they open in different ones: the
// palette is dense in the markup and saved colors is not. The grid id names the button and the
// stored key, so one function does both.
const DENSE_GRIDS = {
	saved: { key: DENSE_KEY, def: false },
	palette: { key: PALETTE_DENSE_KEY, def: true },
};

function applyDense(grid, on) {
	$(grid).classList.toggle("dense", on);
	$(`${grid}-dense`).setAttribute("aria-pressed", String(on));
	writeStored(DENSE_GRIDS[grid].key, on);
}

function denseOn(grid) {
	return $(grid).classList.contains("dense");
}

// Allowed values for every listing control and the harmony color space.
// These mirror the markup and the API.
// Invalid stored values are ignored, so stale preferences cannot produce a 400.
// The array order defines the cycle order.
const CONTROL_VALUES = {
	sort: ["created_at", "hex", "color"],
	order: ["desc", "asc"],
	limit: ["10", "20", "50", "100"],
	"palette-sort": ["name", "hex", "color"],
	"palette-order": ["asc", "desc"],
	space: ["hsl", "oklab"],
};

// A label per value for the controls that are buttons rather than menus. Being listed here is what
// makes a control a cycler, see isCycle below.
const CYCLE_LABELS = {
	sort: { created_at: "date", hex: "hex", color: "color" },
	order: { desc: "desc \u2193", asc: "asc \u2191" },
	"palette-sort": { name: "name", hex: "hex", color: "color" },
	"palette-order": { desc: "desc \u2193", asc: "asc \u2191" },
	space: { hsl: "hsl", oklab: "oklab" },
};

// Clicking advances to the next value and wraps at the end.
// Store the value in data-value; the visible label only displays it.
function initCycle(id, onChange) {
	const el = $(id);
	const values = CONTROL_VALUES[id];
	const labels = CYCLE_LABELS[id];
	const next = () => values[(values.indexOf(el.dataset.value) + 1) % values.length];
	const render = () => {
		el.textContent = labels[el.dataset.value];
		el.title = `switch to ${labels[next()]}`;
	};
	el.addEventListener("click", () => {
		el.dataset.value = next();
		render();
		onChange();
	});
	render();
}

// Cyclers store their value in data-value; menus store it in .value.
// These helpers hide that difference.
function isCycle(id) {
	return id in CYCLE_LABELS;
}

function readControl(id) {
	return isCycle(id) ? $(id).dataset.value : $(id).value;
}

function writeControl(id, value) {
	if (isCycle(id)) {
		$(id).dataset.value = value;
	} else {
		$(id).value = value;
	}
}

function validControl(id, value) {
	return typeof value === "string" && CONTROL_VALUES[id].includes(value);
}

function rememberControls() {
	const state = {};
	for (const id of Object.keys(CONTROL_VALUES)) {
		state[id] = readControl(id);
	}
	writeStored(CONTROLS_KEY, state);
}

// Runs before the toggles are labelled and before the first load, so both read what was restored.
function initControls() {
	const state = readStoredObject(CONTROLS_KEY);
	for (const id of Object.keys(CONTROL_VALUES)) {
		if (validControl(id, state[id])) {
			writeControl(id, state[id]);
		}
	}
}

// app.js is deferred, so the document is complete here. A section with no stored preference keeps
// whatever `open` its markup declares.
function initCollapsibleSections() {
	const state = readStoredObject(DETAILS_KEY);
	for (const section of document.querySelectorAll("details[data-collapse-key]")) {
		const key = section.dataset.collapseKey;
		if (typeof state[key] === "boolean") {
			section.open = state[key];
		}
		section.addEventListener("toggle", () => {
			state[key] = section.open;
			writeStored(DETAILS_KEY, state);
		});
		section.querySelector(":scope > summary").addEventListener("mousedown", (event) => {
			event.preventDefault();
		});
	}
}

// ---- about ----

// Fetched on the first open and kept: the version cannot change under a running page.
let versionLoaded = false;

async function openAbout() {
	$("about").showModal();
	if (versionLoaded) {
		return;
	}
	try {
		const { data } = await call("GET", "/version");
		$("about-version").textContent = data.version;
		versionLoaded = true;
	} catch (err) {
		$("about-version").textContent = "unknown";
		setStatus(err.message, true);
	}
}

// ---- events ----

// Delegated and in the capture phase, so a button built later carries the dip too and a handler
// that stops the click cannot take it away. The class comes off at animationend, so it replays.
document.addEventListener(
	"click",
	(event) => {
		const button = event.target.closest("button");
		if (button === null) {
			return;
		}
		button.addEventListener("animationend", () => button.classList.remove("pressed"), {
			once: true,
		});
		button.classList.add("pressed");
	},
	true,
);

// The title steps through the two tints and then its own color again. The attribute is the only
// record, so a reload starts over.
$("title").addEventListener("click", () => {
	const next = (Number($("title").dataset.tint ?? 0) + 1) % 3;
	if (next === 0) {
		delete $("title").dataset.tint;
	} else {
		$("title").dataset.tint = String(next);
	}
});

$("about-open").addEventListener("click", openAbout);

$("theme").addEventListener("click", (event) => {
	applyTheme(currentTheme() === "dark" ? "light" : "dark");
	if (event.detail > 0) {
		event.currentTarget.blur();
	}
});

$("bg").addEventListener("click", (event) => {
	applyBg(nextBg(currentBg()));
	if (event.detail > 0) {
		event.currentTarget.blur();
	}
});

// detail > 0 is a pointer click. Blurred there, since a focused button takes a focus ring at the
// next keypress, and the key this one is bound to is a keypress the page expects.
$("zen").addEventListener("click", (event) => {
	applyZen(!zenOn());
	if (event.detail > 0) {
		event.currentTarget.blur();
	}
});

/*
	Z toggles it, the one key the page binds. Skipped while a field has the focus, so typing a hex
	or a nickname is not a shortcut. Skipped while a dialog is up too: a key should not reach the
	page behind the backdrop. A modifier means the key belongs to the browser.
*/
document.addEventListener("keydown", (event) => {
	if (event.key !== "z" && event.key !== "Z") {
		return;
	}
	if (event.ctrlKey || event.metaKey || event.altKey) {
		return;
	}
	if (event.target.closest("input, select, textarea, dialog") !== null) {
		return;
	}
	applyZen(!zenOn());
});

document.addEventListener("keydown", (event) => {
	if (event.key !== "l" && event.key !== "L") {
		return;
	}
	if (event.ctrlKey || event.metaKey || event.altKey) {
		return;
	}
	if (event.target.closest("input, select, textarea, dialog") !== null) {
		return;
	}
	applyTheme(currentTheme() === "dark" ? "light" : "dark");
});

document.addEventListener("keydown", (event) => {
	if (event.key !== "b" && event.key !== "B") {
		return;
	}
	if (event.ctrlKey || event.metaKey || event.altKey) {
		return;
	}
	if (event.target.closest("input, select, textarea, dialog") !== null) {
		return;
	}
	applyBg(nextBg(currentBg()));
});

document.addEventListener("keydown", (event) => {
	if (event.key !== "d" && event.key !== "D") {
		return;
	}
	if (event.ctrlKey || event.metaKey || event.altKey) {
		return;
	}
	if (event.target.closest("input, select, textarea, dialog") !== null) {
		return;
	}
	const grid = Object.keys(DENSE_GRIDS).find((name) =>
		$(name).closest("details").matches(":hover"),
	);
	if (grid !== undefined) {
		applyDense(grid, !denseOn(grid));
	}
});

/*
	The full screen of the region under the pointer. 
	Four regions answer: the two
	sections, one pinned strip, and the Selected panel. A strip carries its own name, since the
	blocks are built per harmony rather than listed here. 
	Returns nothing when the pointer is
	somewhere else, or over a Selected panel with no color in it.
*/
function fullscreenUnderPointer() {
	const block = [...document.querySelectorAll("#strips .strip-block")].find((el) =>
		el.matches(":hover"),
	);
	if (block !== undefined) {
		return () => showStripFullscreen(block.dataset.strip);
	}
	if ($("saved").closest("details").matches(":hover")) {
		return showSavedFullscreen;
	}
	if ($("palette").closest("details").matches(":hover")) {
		return showPaletteFullscreen;
	}
	if ($("detail").closest("details").matches(":hover")) {
		return document.querySelector(".detail-color") === null ? undefined : showSelectedFullscreen;
	}
	return undefined;
}

document.addEventListener("keydown", (event) => {
	if (event.key !== "f" && event.key !== "F") {
		return;
	}
	if (event.ctrlKey || event.metaKey || event.altKey) {
		return;
	}
	if (event.target.closest("input, select, textarea, dialog") !== null) {
		return;
	}
	if (document.fullscreenElement !== null) {
		toggleFullscreen(document.fullscreenElement);
		return;
	}
	fullscreenUnderPointer()?.();
});

/*
	"A" is the add of the color under the pointer, same as "F" is for full screen
	A saved cell carries a delete rather than an add, so the key does nothing over the collection
	grid - the same as the glyph there.
*/
function addUnderPointer() {
	return document.querySelector(".cell:hover .add, .band:hover .add, .detail-stack:hover .add");
}

document.addEventListener("keydown", (event) => {
	if (event.key !== "a" && event.key !== "A") {
		return;
	}
	// ctrl+a is select all, and the field guard below does not cover a selection outside one
	if (event.ctrlKey || event.metaKey || event.altKey) {
		return;
	}
	if (event.target.closest("input, select, textarea, dialog") !== null) {
		return;
	}
	// click, not addColor: the button carries the hex and the disabled flag, and the press dip
	// comes with it, so the key looks like what it stands for
	addUnderPointer()?.click();
});

for (const name of DERIVED) {
	if (AXIS[name] !== undefined) {
		$(`pin-${name}`).title = AXIS[name];
	}
	$(`pin-${name}`).addEventListener("click", () => togglePin(name));
}

// the whole row at once, through the same path a single pin takes
$("pins-all").addEventListener("click", () => applyPinned(DERIVED));
$("pins-none").addEventListener("click", () => applyPinned([]));

$("bulk-add").addEventListener("click", addRandomColors);

$("random-hex").addEventListener("click", () => {
	$("hex").value = "#" + randomDigits();
});

$("paste-hex").addEventListener("click", pasteHex);

$("selected-add").addEventListener("click", () => addColor(selectedHex, $("selected-add")));

$("selected-fullscreen").addEventListener("click", showSelectedFullscreen);

for (const tab of ACCOUNT_TABS) {
	$(`tab-${tab}`).addEventListener("click", () => selectAccountTab(tab));
}

$("load-more").addEventListener("click", () => {
	loadSaved({ append: true });
});

// Changing this invalidates the cursor, and loadSaved without append drops it. The two cyclers
// beside it do the same through initCycle, which fires on click rather than change.
$("limit").addEventListener("change", () => {
	rememberControls();
	loadSaved();
});

$("login-open").addEventListener("click", openAccount);

// The dialog closes on the token, not on the nickname: GET /me is a second request, and the panel
// renders nameless until it lands either way.
$("login-form").addEventListener("submit", async (event) => {
	event.preventDefault();
	try {
		await login($("login-nick").value, $("login-password").value);
		$("login-password").value = ""; // the nickname is worth keeping in the field, this is not
	} catch (err) {
		showAccountError(err.message);
		setStatus(err.message, true);
	}
});

// Signing up answers with the user and no token, so the password is spent twice. POST /tokens
// stays the one endpoint that mints one.
$("signup-form").addEventListener("submit", async (event) => {
	event.preventDefault();
	const nickname = $("new-nick").value;
	const password = $("new-password").value;
	try {
		const { data } = await call("POST", "/users", { nickname, password });
		setStatus(`${data.user.nickname} created`);
		await login(nickname, password);
		$("new-nick").value = "";
		$("new-password").value = "";
	} catch (err) {
		showAccountError(err.message);
		setStatus(err.message, true);
	}
});

// Not an <a href> and not a window.open: the token is a header and not a cookie, so the browser
// would ask for this unauthenticated and land on a 401. The answer is fetched like every other
// request and handed back to the browser as a blob.
$("export").addEventListener("click", async () => {
	let answer;
	try {
		answer = await call("GET", "/me/export");
	} catch (err) {
		setStatus(err.message, true);
		return;
	}
	// the page is served by the API, so the header is readable without an expose header
	const name = exportFilename(answer.headers.get("Content-Disposition"));
	const url = URL.createObjectURL(
		new Blob([JSON.stringify(answer.data, null, 2)], { type: "application/json" }),
	);
	const link = document.createElement("a");
	link.href = url;
	link.download = name;
	link.click();
	URL.revokeObjectURL(url);
	setStatus(`exported ${name}`);
});

// The endpoint revokes this token and leaves the account's others alone. Dropping the local copy
// alone is not enough: the token would keep working until it expired.
//
// The session ends either way. A failure means the token may still be live, which is reported.
$("logout").addEventListener("click", async () => {
	try {
		await call("DELETE", "/tokens");
		setStatus("logged out");
	} catch (err) {
		setStatus(`logged out locally, the token may still be live - ${err.message}`, true);
	} finally {
		endSession();
	}
});

// The nickname has to be in hand before the dialog opens, since the field is checked against it.
// A session stored before the nickname was kept spends a GET /me here.
async function openDeleteAccount() {
	if (!requireSession()) {
		return;
	}
	let nickname;
	try {
		nickname = await sessionNickname();
	} catch (err) {
		setStatus(err.message, true);
		return;
	}
	$("account-delete-nick-echo").textContent = nickname;
	$("account-delete-nick").value = "";
	showDeleteError("");
	renderDeleteMatch();
	$("account-delete").showModal();
	$("account-delete-nick").focus();
}

// The server trims and lowercases a nickname before it stores one, so the typed copy gets the
// same treatment before the compare. An empty field matches nothing.
function renderDeleteMatch() {
	const typed = $("account-delete-nick").value.trim().toLowerCase();
	$("account-delete-submit").disabled =
		typed === "" || typed !== $("account-delete-nick-echo").textContent;
}

function showDeleteError(message) {
	$("account-delete-error").textContent = message;
}

// endSession() is last and nothing authed follows it: a request after it answers 401 and logs a
// second line over the first. There is no token to revoke either, that row went with the account.
async function deleteAccount() {
	try {
		await call("DELETE", "/me");
	} catch (err) {
		// on a 401 call() has already ended the session, and the dialog would be standing over a
		// logged out page
		if (session === null) {
			$("account-delete").close();
		} else {
			showDeleteError(err.message);
		}
		setStatus(err.message, true);
		return;
	}
	$("account-delete").close();
	endSession();
	setStatus("account deleted");
}

$("account-delete-open").addEventListener("click", openDeleteAccount);
$("account-delete-nick").addEventListener("input", renderDeleteMatch);
$("account-delete-cancel").addEventListener("click", () => $("account-delete").close());

$("account-delete-form").addEventListener("submit", (event) => {
	event.preventDefault();
	deleteAccount();
});

// Over USER_COLLECTION_QUOTA this answers 409, same as the color quota. The field keeps its value
// on a failure, so the name can be retried once a collection has been freed.
$("collection-form").addEventListener("submit", async (event) => {
	event.preventDefault();
	if (!requireSession()) {
		return;
	}
	try {
		await createCollection($("collection-name").value, selectedIcon, accentHex(selectedAccent));
		openCollectionForm(false); // the new tab is showing, and a second create is rare
	} catch (err) {
		setStatus(err.message, true);
	}
});

// aria-expanded is the record, same as the icon menu. Opening lands the focus in the name field,
// since the button is the only reason the form is up.
function openCollectionForm(open) {
	$("collection-new").setAttribute("aria-expanded", String(open));
	$("collection-form").hidden = !open;
	if (open) {
		$("collection-name").value = randomCollectionName();
		$("collection-name").focus();
		$("collection-name").select();
	}
}

$("collection-new").addEventListener("click", () => {
	openCollectionForm($("collection-new").getAttribute("aria-expanded") !== "true");
});

// Additional controls, folded behind the ellipsis.
function openCollectionMenu(open) {
	$("collection-more").setAttribute("aria-expanded", String(open));
	$("collection-drop").hidden = !open;
}

function collectionMenuOpen() {
	return $("collection-more").getAttribute("aria-expanded") === "true";
}

$("collection-more").addEventListener("click", () => openCollectionMenu(!collectionMenuOpen()));

$("collection-drop").addEventListener("click", () => openCollectionMenu(false));

document.addEventListener("click", (event) => {
	if (collectionMenuOpen() && event.target.closest(".collection-menu") === null) {
		openCollectionMenu(false);
	}
});

document.addEventListener("keydown", (event) => {
	if (event.key === "Escape" && collectionMenuOpen()) {
		openCollectionMenu(false);
		$("collection-more").focus();
	}
});

$("collection-empty").addEventListener("click", emptyCollection);
$("collection-delete").addEventListener("click", deleteCollection);

for (const grid of Object.keys(DENSE_GRIDS)) {
	$(`${grid}-dense`).addEventListener("click", () => applyDense(grid, !denseOn(grid)));
}

$("saved-fullscreen").addEventListener("click", showSavedFullscreen);
$("palette-fullscreen").addEventListener("click", showPaletteFullscreen);

// One grid at a time: the form row has no width for two, and the second would cover the first.
for (const name of MENUS) {
	$(`${name}-trigger`).addEventListener("click", () => {
		const open = !menuOpen(name);
		for (const other of MENUS) {
			openMenu(other, other === name && open);
		}
	});
}

// A trigger is inside its own .picker-menu, so its own click is not an outside one and stays a
// toggle. An option's click closes its menu itself.
document.addEventListener("click", (event) => {
	if (event.target.closest(".picker-menu") !== null) {
		return;
	}
	for (const name of MENUS) {
		if (menuOpen(name)) {
			openMenu(name, false);
		}
	}
});

// Escape closes the menu, and the focus goes back to the trigger rather than to the document.
document.addEventListener("keydown", (event) => {
	if (event.key !== "Escape") {
		return;
	}
	for (const name of MENUS) {
		if (menuOpen(name)) {
			openMenu(name, false);
			$(`${name}-trigger`).focus();
		}
	}
});

$("who").addEventListener("click", () => openAccountMenu(!accountMenuOpen()));

$("account-drop").addEventListener("click", () => openAccountMenu(false));

// Same shape as the icon menu.
document.addEventListener("click", (event) => {
	if (accountMenuOpen() && event.target.closest(".account-menu") === null) {
		openAccountMenu(false);
	}
});

document.addEventListener("keydown", (event) => {
	if (event.key === "Escape" && accountMenuOpen()) {
		openAccountMenu(false);
		$("who").focus();
	}
});

// Adding is idempotent: 201 means it was new, 200 means the user already had it.
$("add-form").addEventListener("submit", async (event) => {
	event.preventDefault();
	if (!requireSession()) {
		return;
	}
	const hex = parseHex($("hex").value);
	if (hex === null) {
		setStatus("type a hex first", true);
		return;
	}
	try {
		const { status, data } = await call("POST", await savedColorsPath(), { hex });
		setStatus(status === 201 ? `added ${data.hex}` : `${data.hex} was already saved`);
		await loadSaved();
	} catch (err) {
		setStatus(err.message, true);
	}
});

// ---- start ----
// The theme attribute is already stamped by the inline script, this only labels the button.

renderThemeButton(currentTheme());
// the <head> script stamped the attribute already, this labels the button. A stored value outside
// the three is dropped here (same as with a stored control outside CONTROL_VALUES).
applyBg(BG_VALUES.includes(currentBg()) ? currentBg() : "off");
applyZen(readStored(ZEN_KEY, false) === true);
for (const [grid, { key, def }] of Object.entries(DENSE_GRIDS)) {
	applyDense(grid, readStored(key, def) === true);
}
initCollapsibleSections();
loadSession(); // before renderSession and the first loadSaved, both of which read it
initControls(); // before the cyclers below, which label themselves from data-value
for (const id of ["sort", "order"]) {
	initCycle(id, () => {
		rememberControls();
		loadSaved();
	});
}
for (const id of ["palette-sort", "palette-order"]) {
	initCycle(id, () => {
		rememberControls();
		loadPalette();
	});
}
// The wheel changes what every rotation answers, so the strips reload. A closed section records
// that it skipped this the way it does a selection.
initCycle("space", () => {
	rememberControls();
	showStrips();
});

initPicker();
initScratch(); // labels the hex button from the input's own value in the markup
$("harmony-section").addEventListener("toggle", () => {
	if ($("harmony-section").open && stripsStale) {
		showStrips();
	}
});
// after initCollapsibleSections, which decides whether the section it draws into is open. A
// stored value that is not an array is dropped, the way a stored control outside CONTROL_VALUES is.
const storedPinned = readStored(PINNED_KEY, DEF_PINNED);
applyPinned(Array.isArray(storedPinned) ? storedPinned : DEF_PINNED);
// fills the Selected panel and every pinned strip, one request each. The grids mark the swatch
// themselves once they land, see renderSwatches.
const opening = storedSelection();
selectColor(opening.hex, opening.name);
renderLog(); // same, until something happens
renderIconPicker(); // reads the sprite once, before anything can select out of it
renderAccentPicker(); // reads the --accent-* tokens once
renderCollections(); // the empty list, until the first response replaces it
renderSession(); // picks the account panel, and hides the two sections that need a token
loadPalette(); // public, so it runs logged out too
if (session !== null) {
	loadSaved(); // pulls the collection list on its way, see savedColorsPath
}
