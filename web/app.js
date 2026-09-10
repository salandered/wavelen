// Everything the page knows about the service is in api.yaml. The api binary embeds this page and
// serves it beside the API, so the path is relative and no CORS header exists anywhere.
const API = "/api/v1";

// "theme" is written by the inline script in <head> and stays a raw string. Everything read
// through readStored below is JSON, so the two don't share a key.
const DETAILS_KEY = "details_open";
const SWATCH_INFO_KEY = "swatch_info";
const SESSION_KEY = "session";
const CONTROLS_KEY = "controls";

const $ = (id) => document.getElementById(id);

// ---- browser storage ----
// Optional everywhere: a private window or blocked site data leaves the page working without
// preferences.

function readStored(key, fallback) {
	try {
		const raw = localStorage.getItem(key);
		return raw === null ? fallback : JSON.parse(raw);
	} catch {
		return fallback;
	}
}

// A stored JSON object, or {} for anything else in the key. Two callers keep a map under one key,
// and a hand-edited value is dropped here rather than hit later.
function readStoredObject(key) {
	const stored = readStored(key, null);
	const usable = stored !== null && typeof stored === "object" && !Array.isArray(stored);
	return usable ? stored : {};
}

function writeStored(key, value) {
	try {
		localStorage.setItem(key, JSON.stringify(value));
	} catch {
		// nothing to do, the preference doesn't survive the reload
	}
}

// ---- session ----
// Four fields: the token and its expiry from POST /tokens, the name from GET /me, and the id of
// the collection the saved grid is showing. Both of the last two are stored rather than re-fetched
// so a reload renders without a request, and both are safe stale - the name is a caption, and the
// id is checked against the list before it is used.
//
// See web-wavelen-context.md, "Storage", for why the token is in localStorage.

let session = null;

// An expired session is dropped rather than sent, which only saves the doomed request. The 401
// handling in request() is the actual check.
//
// The collection id is not validated here: pickCollection compares it against the list the account
// has, so anything else in the key falls through to the default.
function loadSession() {
	const stored = readStored(SESSION_KEY, null);
	const usable = stored !== null
		&& typeof stored === "object"
		&& typeof stored.token === "string"
		&& typeof stored.expiry === "string"
		&& typeof stored.name === "string"
		&& Date.parse(stored.expiry) > Date.now();

	session = usable ? stored : null;
	if (!usable) {
		writeStored(SESSION_KEY, null);
	}
}

function startSession(token, expiry, name) {
	session = { token, expiry, name };
	resetCollections(); // the previous account's list and active id belong to nothing now
	writeStored(SESSION_KEY, session);
	renderSession();
}

// The name arrives one request after the token, see login().
function setSessionName(name) {
	session = { ...session, name };
	writeStored(SESSION_KEY, session);
	renderSession();
}

// Local only. Revoking is the caller's business: logout does it, an expiry or a 401 means it is
// already done.
function endSession() {
	session = null;
	resetCollections();
	writeStored(SESSION_KEY, null);
	renderSession();
}

// ---- requests ----

// Errors come back as {"error": "..."}, but a proxy or a panic can still produce a non-JSON body
// with an error status, so the body is read as text and parsed opportunistically.
//
// The token goes on every request once there is one, public paths included. They ignore it, and
// one rule here beats a flag at each call site.
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

	// There is no refresh, so a 401 with a token in hand means that token is finished: expired, or
	// revoked here or from another tab. Going back to logged out beats leaving controls that
	// answer 401 on every click.
	if (res.status === 401 && session !== null) {
		endSession();
	}
	if (!res.ok) {
		throw new Error(`${res.status} - ${data?.error ?? text}`);
	}
	return { status: res.status, data };
}

// ---- log ----

// The last few things that happened, newest first. In memory only: it says what this page just did,
// which would be a lie after a reload.
const LOG_LIMIT = 10;
const logEntries = [];

// A transient entry is a live counter, so the next entry replaces it instead of landing under it.
// The bulk run uses it to rewrite one line rather than fill the panel with its progress.
function pushLog(message, { failed = false, transient = false } = {}) {
	if (message === "") {
		return; // nothing happened, and an empty entry would push a real one out of the panel
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
	$("log").replaceChildren(...logEntries.map((entry) => {
		const line = document.createElement("p");
		line.textContent = entry.message;
		line.classList.toggle("failed", entry.failed);
		return line;
	}));
}

function setStatus(message, failed = false) {
	pushLog(message, { failed });
}

function setProgress(message) {
	pushLog(message, { transient: true });
}

// /me/... resolves the user from the token server-side, so this only stops a logged out click from
// sending a request that could only answer 401.
function requireSession() {
	if (session === null) {
		setStatus("log in first", true);
		return false;
	}
	return true;
}

// ---- collections ----
// Every saved-colors path carries a collection id, so the page asks GET /me/collections for one.
// The list is fetched once per session and kept, so a create or a delete edits the local copy
// rather than re-asking. See web-wavelen-context.md, "Collections".

let collections = [];
let activeCollection = null;

// The request, kept rather than its answer, so the bulk-add workers share one lookup.
let collectionsRequest = null;

function ensureCollections() {
	collectionsRequest ??= loadCollections().catch((err) => {
		// cleared, so the next click asks again instead of replaying the failure
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

// The stored one if the account still has it, else the default, else the oldest, else null.
function pickCollection(preferred) {
	const found = collections.find((c) => c.id === preferred)
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
// so it has to die when the session does.
function setActiveCollection(id) {
	activeCollection = id;
	session = { ...session, collection: id };
	writeStored(SESSION_KEY, session);
	renderCollections();
}

// Switching invalidates the cursor, which was minted against the rows of the collection being
// left. loadSaved without append drops it.
function selectCollection(id) {
	if (id === activeCollection) {
		return;
	}
	setActiveCollection(id);
	setStatus(`showing ${collectionName(id)}`);
	loadSaved();
}

// The next login unhides the section before its own request lands, which would otherwise show one
// account the previous one's list.
function resetCollections() {
	collections = [];
	activeCollection = null;
	collectionsRequest = null;
	renderCollections();
}

// ---- collection icons ----
// The set is the sprite in index.html and nowhere else on this page: the picker is built by reading
// the symbol ids back out. The server holds the same list in internal/icon and answers 400 for a
// slug outside it.
const DEF_ICON = "folder";
const DEF_ACCENT = "#808080";

let selectedIcon = DEF_ICON;

function iconNames() {
	return [...document.querySelectorAll("#icon-sprite symbol")]
		.map((symbol) => symbol.id.replace(/^i-/, ""));
}

// An <svg> is not an HTML element, so it and its <use> are created in the SVG namespace or the
// browser parses them as unknown tags and draws nothing.
const SVG_NS = "http://www.w3.org/2000/svg";

function iconSvg(name, className) {
	const svg = document.createElementNS(SVG_NS, "svg");
	svg.setAttribute("class", className);
	svg.setAttribute("aria-hidden", "true");

	const use = document.createElementNS(SVG_NS, "use");
	use.setAttribute("href", `#i-${name}`);
	svg.append(use);
	return svg;
}

// Built once: the sprite is static, and only the selected mark and the tint change afterwards.
function renderIconPicker() {
	$("icon-picker").replaceChildren(...iconNames().map((name) => {
		const option = document.createElement("button");
		option.type = "button";
		option.className = "icon-option";
		option.dataset.icon = name;
		option.title = name;
		option.setAttribute("role", "radio");
		option.append(iconSvg(name, "icon"));
		option.addEventListener("click", () => {
			selectIcon(name);
			openIconMenu(false);
		});
		return option;
	}));
	selectIcon(DEF_ICON);
}

function selectIcon(name) {
	selectedIcon = name;
	for (const option of $("icon-picker").children) {
		option.setAttribute("aria-checked", String(option.dataset.icon === name));
	}
	renderIconChoice();
}

// Both the trigger and the grid carry the accent, so the glyphs are compared in the color a create
// at this moment would use.
function renderIconChoice() {
	const accent = $("collection-accent").value;
	$("icon-picker").style.color = accent;
	$("icon-trigger").style.color = accent;

	const caret = document.createElement("span");
	caret.className = "caret";
	caret.textContent = "▾";

	$("icon-trigger").replaceChildren(iconSvg(selectedIcon, "icon"), caret);
	// the glyph is the whole label, so the name is spelled out
	$("icon-trigger").title = `icon: ${selectedIcon}`;
	$("icon-trigger").setAttribute("aria-label", `collection icon: ${selectedIcon}`);
}

// aria-expanded is the only record of whether the menu is up. The [hidden] rule turns `hidden`
// into display:none, so the grid can declare a display of its own.
function openIconMenu(open) {
	$("icon-trigger").setAttribute("aria-expanded", String(open));
	$("icon-picker").hidden = !open;
}

function iconMenuOpen() {
	return $("icon-trigger").getAttribute("aria-expanded") === "true";
}

// One chip per collection: the name selects it, the "x" deletes it. The default chip carries the
// marker instead, because the server refuses that delete with a 409.
function renderCollections() {
	$("saved-in").textContent = collectionName(activeCollection);

	if (collections.length === 0) {
		renderEmpty($("collections"), "none");
		return;
	}
	$("collections").replaceChildren(...collections.map((col) => {
		const row = document.createElement("div");
		row.className = "collection";
		row.classList.toggle("active", col.id === activeCollection);

		// an account made before the icon existed has neither field, so both fall back
		const glyph = iconSvg(col.icon ?? DEF_ICON, "icon collection-icon");
		glyph.style.color = col.accent ?? DEF_ACCENT;
		row.append(glyph);

		const name = document.createElement("button");
		name.type = "button";
		name.className = "collection-name";
		name.textContent = col.name;
		name.title = col.id === activeCollection ? "showing this one" : `show ${col.name}`;
		name.addEventListener("click", () => selectCollection(col.id));
		row.append(name);

		row.append(col.is_default ? defaultTag() : collectionRemove(col));
		return row;
	}));
}

function defaultTag() {
	const tag = document.createElement("span");
	tag.className = "tag";
	tag.textContent = "default";
	tag.title = "written at signup, and the one collection that cannot be deleted";
	return tag;
}

function collectionRemove(col) {
	const remove = document.createElement("button");
	remove.type = "button";
	remove.className = "collection-delete";
	remove.textContent = "×";
	remove.title = `delete ${col.name} and its colors`;
	remove.setAttribute("aria-label", `delete ${col.name}`);
	remove.addEventListener("click", () => deleteCollection(col));
	return remove;
}

// The one control that destroys rows it is not showing: the delete cascades to the colors and
// there is no account recovery. Hence the confirm, which deleting a single swatch does not get.
async function deleteCollection(col) {
	if (!confirm(`delete ${col.name} and every color in it?`)) {
		return;
	}
	try {
		await call("DELETE", `/me/collections/${col.id}`);
		collections = collections.filter((c) => c.id !== col.id);
		setStatus(`deleted ${col.name}`);

		if (col.id !== activeCollection) {
			renderCollections();
			return;
		}
		// the grid is showing a collection that no longer exists
		setActiveCollection(pickCollection(null));
		await loadSaved();
	} catch (err) {
		setStatus(err.message, true);
	}
}

// The list is ordered oldest first, which is where a new row belongs.
async function createCollection(name, icon, accent) {
	await ensureCollections(); // the row goes onto a list, so there has to be one
	const { data } = await call("POST", "/me/collections", { name, icon, accent });
	collections.push(data.collection);
	setStatus(`${data.collection.name} created`);
	selectCollection(data.collection.id);
}

// The caption is one line that has to fit a swatch, so the year is 2-digit where toLocaleString's
// is 4. The locale keeps its own order.
const savedTime = new Intl.DateTimeFormat(undefined, {
	year: "2-digit",
	month: "2-digit",
	day: "2-digit",
	hour: "2-digit",
	minute: "2-digit",
	second: "2-digit",
});

// The comma between date and time is dropped, since the space already reads as the break. Through
// formatToParts rather than off the formatted string, so only the format's own literals change.
function savedLabel(at) {
	return savedTime
		.formatToParts(at)
		.map((part) => (part.type === "literal" ? part.value.replace(",", "") : part.value))
		.join("");
}

// ---- swatches ----

// The selection: one hex and one caption, either a palette name or a saved timestamp. Marks the
// swatch in both grids and fills the aside. Dropped as soon as the hex field says something else.
let selectedHex = null;
let selectedLabel = "";

// Perceived brightness (the YIQ weights) picks between a black and a white label. Not a contrast
// ratio, but one line and enough to keep every swatch readable.
function labelColor(hex) {
	const n = parseInt(hex.slice(1), 16);
	const r = (n >> 16) & 0xff;
	const g = (n >> 8) & 0xff;
	const b = n & 0xff;
	return (r * 299 + g * 587 + b * 114) / 1000 > 140 ? "#000" : "#fff";
}

function markSelected() {
	for (const el of document.querySelectorAll(".swatch")) {
		el.classList.toggle("selected", el.dataset.hex === selectedHex);
	}
}

function swatch(hex, label) {
	const el = document.createElement("button");
	el.type = "button";
	el.className = "swatch";
	el.dataset.hex = hex;
	el.style.background = hex;
	el.style.color = labelColor(hex);
	el.title = `use ${hex}`;

	const code = document.createElement("span");
	code.textContent = hex;
	const caption = document.createElement("span");
	caption.className = "label";
	caption.textContent = label;

	el.append(code, caption);
	el.addEventListener("click", () => selectColor(hex, label));
	return el;
}

// The picker mirrors the selection, so it lands on a color chosen anywhere else and can nudge it.
//
// Everything selects through here except the picker itself, which goes through setSelection: it
// already holds the value, and writing it back into the input while its dialog is open would fight
// the dialog for it.
function selectColor(hex, label) {
	$("pick").value = hex;
	renderPick();
	setSelection(hex, label);
}

// Every producer comes through here, so the add field and the panels beside it cannot disagree.
function setSelection(hex, label) {
	selectedHex = hex;
	selectedLabel = label;
	$("hex").value = hex; // the add form's field, so "add" saves what the panels are showing
	selectionChanged();
}

function clearSelection() {
	selectedHex = null;
	selectedLabel = "";
	selectionChanged();
}

// Both aside panels show the selected color, so nothing moves only one of them.
function selectionChanged() {
	markSelected();
	renderDetail();
	$("complement").disabled = selectedHex === null;
	$("triad").disabled = selectedHex === null;

	// a strip built for the previous color would be wrong, so it either follows or goes back to
	// the placeholder
	if (harmonyMode !== null && selectedHex !== null) {
		showHarmony(harmonyMode);
	} else {
		renderHarmonyPlaceholder();
	}
}

// A saved swatch carries a delete control, a palette one does not. Siblings inside a cell, not one
// inside the other: the cell is what the grid lays out and what the hover rule keys off.
function savedSwatch(hex, label) {
	const cell = document.createElement("div");
	cell.className = "cell";

	const remove = document.createElement("button");
	remove.type = "button";
	remove.className = "remove";
	remove.textContent = "×";
	remove.style.color = labelColor(hex); // it sits on the color, like the hex does
	remove.title = `delete ${hex}`;
	remove.setAttribute("aria-label", `delete ${hex}`);
	remove.addEventListener("click", () => deleteColor(hex, cell, remove));

	cell.append(swatch(hex, label), remove);
	return cell;
}

// The path takes the six digits bare: api.yaml rejects a '#' however it is escaped, and an
// unescaped one would be a fragment and never leave the browser.
//
// The cell is dropped rather than the list reloaded, since a reload would drop every appended page.
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

function renderEmpty(container, message) {
	const p = document.createElement("p");
	p.className = "empty";
	p.textContent = message;
	container.replaceChildren(p);
}

// The click is the user gesture the Fullscreen API needs, but an iframe without allowfullscreen or
// a browser policy can still refuse. Nothing is broken by a refusal, so it is only reported.
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

// navigator.clipboard exists only in a secure context, so over plain http the property is missing
// and reading through it throws. In an async function that is a rejection like any other.
async function copyHex(hex) {
	try {
		await navigator.clipboard.writeText(hex);
		setStatus(`copied ${hex}`);
	} catch (err) {
		setStatus(`could not copy - ${err.message}`, true);
	}
}

// One panel for both grids, since the selection they share is one hex.
function renderDetail() {
	if (selectedHex === null) {
		renderEmpty($("detail"), "click a color");
		return;
	}
	const hex = selectedHex; // captured, so a later selection does not rewrite these handlers

	// siblings, not one inside the other, so neither click has to be kept from reaching the other
	const stack = document.createElement("div");
	stack.className = "detail-stack";

	// no text of its own, so the name is spelled out
	const block = document.createElement("button");
	block.type = "button";
	block.className = "detail-color";
	block.style.background = hex;
	block.title = "full screen, click again to leave";
	block.setAttribute("aria-label", `show ${hex} full screen`);
	block.addEventListener("click", () => toggleFullscreen(block));

	const code = document.createElement("button");
	code.type = "button";
	code.className = "detail-hex";
	code.style.color = labelColor(hex);
	code.textContent = hex;
	code.title = "copy";
	code.addEventListener("click", () => copyHex(hex));

	stack.append(block, code);

	const caption = document.createElement("p");
	caption.className = "detail-label";
	caption.textContent = selectedLabel;

	$("detail").replaceChildren(stack, caption);
}

// ---- color picker ----
// <input type="color"> is the whole picker: the dialog belongs to the browser, and this panel is
// the swatch that opens it. What it produces is a selection, the same as a swatch click.
//
// A drag reports every color it passes through, and with a harmony on screen each selection is a
// request. So the label follows every step and the selection waits for the drag to go quiet.
const PICK_QUIET = 200;

let pickTimer = null;

function renderPick() {
	$("pick-hex").textContent = $("pick").value;
}

function initPicker() {
	// input reports each step of a drag, change the committed value. Which of them a browser sends
	// and how often varies, so both schedule the same commit and the timer collapses the gesture.
	for (const type of ["input", "change"]) {
		$("pick").addEventListener(type, () => {
			renderPick();
			clearTimeout(pickTimer);
			pickTimer = setTimeout(() => setSelection($("pick").value, "picked"), PICK_QUIET);
		});
	}
	$("pick-hex").addEventListener("click", () => copyHex($("pick").value));
	renderPick();
}

// ---- harmony ----

// Which harmony the panel is showing, so clicking a different swatch keeps it rather than making
// the button be pressed again.
let harmonyMode = null;

// Same guard as loadSaved: the buttons stay live while a request is out, so an earlier response
// landing later must not replace a fresher strip.
let harmonyGeneration = 0;

// The endpoint answers with the color asked about beside the others, and the strip shows all of
// them: the pairing is the point.
const HARMONIES = {
	complement: (data) => [data.hex, data.complement],
	triad: (data) => [data.hex, ...data.triad],
};

function renderHarmonyPlaceholder() {
	renderEmpty($("harmony"), selectedHex === null ? "click a color" : "pick a harmony");
}

// Only reached with a selection: both buttons are disabled without one.
async function showHarmony(mode) {
	harmonyMode = mode;
	const generation = ++harmonyGeneration;
	const hex = selectedHex;

	try {
		// bare six digits in the path, same rule as the delete above
		const { data } = await call("GET", `/colors/${hex.slice(1)}/${mode}`);
		if (generation !== harmonyGeneration) {
			return; // a later click owns the panel now
		}
		renderHarmony(HARMONIES[mode](data));
	} catch (err) {
		if (generation !== harmonyGeneration) {
			return;
		}
		renderHarmonyPlaceholder();
		setStatus(err.message, true);
	}
}

// The strip goes full screen, not a band: one color of a harmony full screen is what the Selected
// panel already does. So the strip is a plain element and every band carries its own pair.
function renderHarmony(hexes) {
	const strip = document.createElement("div");
	strip.className = "harmony";
	// every band opens the same strip, so they share one label rather than each naming its color
	const fullscreenLabel = `show ${hexes.join(" ")} full screen`;

	for (const hex of hexes) {
		const band = document.createElement("div");
		band.className = "band";

		const block = document.createElement("button");
		block.type = "button";
		block.className = "band-color";
		block.style.background = hex;
		block.title = "full screen, click again to leave";
		block.setAttribute("aria-label", fullscreenLabel);
		block.addEventListener("click", () => toggleFullscreen(strip));

		const code = document.createElement("button");
		code.type = "button";
		code.className = "band-hex";
		code.style.color = labelColor(hex);
		code.textContent = hex;
		code.title = "copy";
		code.addEventListener("click", () => copyHex(hex));

		band.append(block, code);
		strip.append(band);
	}
	$("harmony").replaceChildren(strip);
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
		renderSwatches($("palette"), data.colors.map((c) => swatch(c.hex, c.name)));
	} catch (err) {
		renderEmpty($("palette"), err.message);
	}
}

// The cursor for the next page of the saved list, or null at the end. It carries the sort and the
// order it was minted under, so the API rejects it once either changes. It carries no user, so
// endSession drops it by hand.
let nextCursor = null;

// loadSaved can be in flight more than once, since the controls stay live while a request is out.
// Each call takes the next number and only touches the grid while its number is still current, so
// a request that started earlier and landed later cannot overwrite a fresher one.
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

	// nextCursor holds until the response replaces it, so a second click while this one is out
	// would send it again and append the same page twice
	$("load-more").disabled = true;
	try {
		const { data } = await call("GET", `${await savedColorsPath()}?${savedQuery(cursor)}`);
		if (generation !== savedGeneration) {
			return; // a later load owns the grid now
		}
		const swatches = data.colors.map((c) =>
			savedSwatch(c.hex, savedLabel(new Date(c.created_at))));

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
// For poking at the API by hand. Math.random is enough: nothing here is a secret, and a collision
// is a color the account already saved, which answers 200 instead of 201.
function randomDigits() {
	return Math.floor(Math.random() * 0x1000000).toString(16).padStart(6, "0");
}

// ---- bulk add ----
// Dev only. There is no bulk endpoint: these are ordinary POSTs, a few in flight at a time. All at
// once is a burst nothing else on this page produces, one at a time is a round trip each.

const BULK_COUNT = 20;
const BULK_IN_FLIGHT = 8;

async function addRandomColors() {
	if (!requireSession()) {
		return;
	}

	// Resolved once, before any worker starts. A failed lookup ends the run here rather than
	// failing all twenty adds with the same message.
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
	const hexes = [...queue];

	let done = 0;
	let created = 0;
	let failed = 0;
	let firstError = null;

	// each worker drains the same array, so a slow request doesn't hold up the others
	const worker = async () => {
		while (hexes.length > 0) {
			const hex = hexes.pop();
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
		}
	};

	$("bulk-add").disabled = true;
	try {
		await Promise.all(Array.from({ length: BULK_IN_FLIGHT }, worker));
	} finally {
		$("bulk-add").disabled = false;
	}

	// created counts 201s only, so the rest were colors this user already had
	const summary = `added ${created} of ${BULK_COUNT}`;
	setStatus(failed === 0 ? summary : `${summary}, ${failed} failed - ${firstError}`, failed > 0);
	await loadSaved();
}

// ---- account ----

// Both forms ask for a nickname and a password, so only one is on screen. The choice is not stored:
// the panel only exists logged out.
const ACCOUNT_TABS = ["login", "signup"];

// aria-selected is the record, and the CSS reads it: nothing else tracks which tab is up.
function selectAccountTab(name) {
	for (const tab of ACCOUNT_TABS) {
		const selected = tab === name;
		$(`tab-${tab}`).setAttribute("aria-selected", String(selected));
		$(`${tab}-form`).hidden = !selected;
	}
}

// Only Collections and Saved colors need a token. The palette and both harmonies are public, so a
// logged out visitor keeps a working page.
function renderSession() {
	$("logged-out").hidden = session !== null;
	$("logged-in").hidden = session === null;
	$("saved-section").hidden = session === null;
	$("collections-section").hidden = session === null;

	if (session === null) {
		// the last visit may have left the panel on sign up
		selectAccountTab("login");
		// the cursor was minted for the session that just ended and carries no user of its own,
		// so a later load-more would append the previous account's next page
		setNextCursor(null);
		// the grid still holds that account's swatches, and the next login unhides this section
		// before its own request lands
		renderEmpty($("saved"), "nothing saved");
		return;
	}
	// empty while GET /me is in flight, and after it failed
	$("who").textContent = session.name || "logged in";
}

// Two requests: the token, then the account it belongs to. GET /me is authenticated, so the session
// has to exist before the name can be asked for, and the panel renders nameless until it lands.
async function login(nickname, password) {
	const { data } = await call("POST", "/tokens", { nickname, password });
	startSession(data.token, data.expiry, "");

	const { data: me } = await call("GET", "/me");
	setSessionName(me.user.name);
	setStatus(`logged in as ${me.user.name}`);
	await loadSaved();
}

// ---- preferences ----

function currentTheme() {
	return document.documentElement.dataset.theme
		?? (matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light");
}

function applyTheme(theme) {
	document.documentElement.dataset.theme = theme;
	$("theme").textContent = theme === "dark" ? "light" : "dark";
	try {
		// raw, not writeStored: the <head> script reads this one back without parsing it
		localStorage.setItem("theme", theme);
	} catch {
		// the preference doesn't survive a reload
	}
}

function applySwatchInfo(hidden) {
	document.documentElement.classList.toggle("no-swatch-info", hidden);
	$("swatch-info").textContent = hidden ? "show info" : "hide info";
	writeStored(SWATCH_INFO_KEY, hidden);
}

// Every listing control and the values the API accepts, mirroring the markup. Anything not listed
// is dropped when a stored preference is read, so a stale key cannot produce a 400. The order here
// is the order a cycler steps through.
const CONTROL_VALUES = {
	"sort": ["created_at", "hex", "color"],
	"order": ["desc", "asc"],
	"limit": ["10", "20", "50", "100"],
	"palette-sort": ["name", "hex", "color"],
	"palette-order": ["asc", "desc"],
};

// A label per value for the controls that are buttons rather than menus. Being listed here is what
// makes a control a cycler, see isCycle below.
const CYCLE_LABELS = {
	"sort": { created_at: "date", hex: "hex", color: "color" },
	"order": { desc: "desc \u2193", asc: "asc \u2191" },
	"palette-sort": { name: "name", hex: "hex", color: "color" },
	"palette-order": { desc: "desc \u2193", asc: "asc \u2191" },
};

// A click steps to the next value and wraps. The value lives in data-value, which is where the
// query builders read it; the label only reports it.
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

// A cycler keeps its value in data-value and a menu in .value. That is the only difference between
// the two, and these three functions are where it lives.
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
	}
}

// ---- events ----

$("theme").addEventListener("click", () => {
	applyTheme(currentTheme() === "dark" ? "light" : "dark");
});

$("swatch-info").addEventListener("click", () => {
	applySwatchInfo(!document.documentElement.classList.contains("no-swatch-info"));
});

// a mark that no longer matches the field would name a color other than the one about to be added
$("hex").addEventListener("input", () => {
	if (selectedHex !== null && $("hex").value !== selectedHex) {
		clearSelection();
	}
});

for (const mode of Object.keys(HARMONIES)) {
	$(mode).addEventListener("click", () => showHarmony(mode));
}

$("bulk-add").addEventListener("click", addRandomColors);

// a selection like any other, so the panels show what the button produced
$("random-hex").addEventListener("click", () => {
	selectColor("#" + randomDigits(), "random");
});

for (const tab of ACCOUNT_TABS) {
	$(`tab-${tab}`).addEventListener("click", () => selectAccountTab(tab));
}

$("load").addEventListener("click", () => {
	loadSaved();
});

$("load-more").addEventListener("click", () => {
	loadSaved({ append: true });
});

// Changing this invalidates the cursor, and loadSaved without append drops it. The two cyclers
// beside it do the same through initCycle, which fires on click rather than change.
$("limit").addEventListener("change", () => {
	rememberControls();
	loadSaved();
});

$("login-form").addEventListener("submit", async (event) => {
	event.preventDefault();
	try {
		await login($("login-nick").value, $("login-password").value);
		$("login-password").value = ""; // the nickname is worth keeping in the field, this is not
	} catch (err) {
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
		const { data } = await call("POST", "/users", {
			nickname,
			name: $("new-name").value,
			password,
		});
		setStatus(`${data.user.name} created`);
		await login(nickname, password);
		$("new-nick").value = "";
		$("new-name").value = "";
		$("new-password").value = "";
	} catch (err) {
		setStatus(err.message, true);
	}
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

// Over USER_COLLECTION_QUOTA this answers 409, same as the color quota. The field keeps its value
// on a failure, so the name can be retried once a collection has been freed.
$("collection-form").addEventListener("submit", async (event) => {
	event.preventDefault();
	if (!requireSession()) {
		return;
	}
	try {
		await createCollection(
			$("collection-name").value, selectedIcon, $("collection-accent").value);
		$("collection-name").value = "";
	} catch (err) {
		setStatus(err.message, true);
	}
});

// No debounce, unlike the picker in the aside: this only writes a style property.
$("collection-accent").addEventListener("input", renderIconChoice);

$("icon-trigger").addEventListener("click", () => openIconMenu(!iconMenuOpen()));

// The trigger is inside .icon-menu, so its own click is not an outside one and stays a toggle. An
// option's click closes the menu itself.
document.addEventListener("click", (event) => {
	if (iconMenuOpen() && event.target.closest(".icon-menu") === null) {
		openIconMenu(false);
	}
});

// Escape closes the menu, and the focus goes back to the trigger rather than to the document.
document.addEventListener("keydown", (event) => {
	if (event.key === "Escape" && iconMenuOpen()) {
		openIconMenu(false);
		$("icon-trigger").focus();
	}
});

// Adding is idempotent: 201 means it was new, 200 means the user already had it.
$("add-form").addEventListener("submit", async (event) => {
	event.preventDefault();
	if (!requireSession()) {
		return;
	}
	try {
		const { status, data } = await call("POST", await savedColorsPath(), { hex: $("hex").value });
		setStatus(status === 201 ? `added ${data.hex}` : `${data.hex} was already saved`);
		await loadSaved();
	} catch (err) {
		setStatus(err.message, true);
	}
});

// ---- start ----
// The theme attribute is already stamped by the inline script, this only labels the button.

$("theme").textContent = currentTheme() === "dark" ? "light" : "dark";
applySwatchInfo(readStored(SWATCH_INFO_KEY, false) === true);
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

initPicker(); // labels the hex button from the input's own value in the markup
renderDetail(); // the placeholder, until a swatch is clicked
renderHarmonyPlaceholder(); // same, and the buttons stay disabled until then
renderLog(); // same, until something happens
renderIconPicker(); // reads the sprite once, before anything can select out of it
renderCollections(); // the empty list, until the first response replaces it
renderSession(); // picks the account panel, and hides the two sections that need a token
loadPalette(); // public, so it runs logged out too
if (session !== null) {
	loadSaved(); // pulls the collection list on its way, see savedColorsPath
}
