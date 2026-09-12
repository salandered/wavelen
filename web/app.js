import { exportFilename, labelColor, parseHex, randomDigits, savedLabel } from "./lib.js";

// Everything the page knows about the service is in api.yaml. The api binary embeds this page and
// serves it beside the API, so the path is relative and no CORS header exists.
const API = "/api/v1";

// "theme" is written by the inline script in <head> and stays a raw string. Everything read
// through readStored below is JSON, so the two don't share a key.
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
// The session: the token and its expiry from POST /tokens, the nickname from GET /me, and the id
// of the collection the saved grid is showing. The last two are stored rather than re-fetched, so
// a reload renders without a request.
//
// They are safe stale: the id is checked against the list before it is used, and the nickname is a
// caption plus the word compared against what was typed into a field the account's owner is
// looking at.
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
	resetCollections(); // the previous account's list and active id belong to nothing now
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

// A session whose GET /me never landed has a token and no nickname, and stays usable. The caption
// renders without one, so the delete dialog, which cannot, asks for it.
async function sessionNickname() {
	if (typeof session?.nickname === "string" && session.nickname !== "") {
		return session.nickname;
	}
	const { data } = await call("GET", "/me");
	setSessionNickname(data.user.nickname);
	return data.user.nickname;
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
const DEF_ICON = "square";
const DEF_ACCENT = "#808080";

let selectedIcon = DEF_ICON;

// i- only: the sprite also holds ui- glyphs the page uses for itself, and internal/icon answers
// 400 for a slug it does not have.
function iconNames() {
	return [...document.querySelectorAll("#icon-sprite symbol[id^='i-']")].map((symbol) =>
		symbol.id.replace(/^i-/, ""),
	);
}

// An <svg> is not an HTML element, so it and its <use> are created in the SVG namespace or the
// browser parses them as unknown tags and draws nothing.
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
				openIconMenu(false);
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

// One tab per collection, and the tab only selects: the delete is the trash in the listing row,
// beside the eraser, and both act on the active one. The default is not marked on its tab; its
// title says so, and the trash is disabled while it is the active one, since the server refuses
// that delete with a 409.
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

			// an account made before the icon existed has neither field, so both fall back
			const glyph = iconSvg(col.icon ?? DEF_ICON, "icon collection-icon");
			glyph.style.color = col.accent ?? DEF_ACCENT;
			const text = document.createElement("span");
			text.className = "name";
			text.textContent = col.name;
			name.append(glyph, text);
			row.append(name);
			return row;
		}),
	);
}

// One of the two controls that destroy rows the page is not showing: the delete cascades to the
// colors and there is no account recovery. Hence the confirm, which deleting a swatch does not get.
// It takes the active collection, the one the grid is showing, so what goes is on screen.
async function deleteCollection() {
	if (!requireSession()) {
		return;
	}
	await ensureCollections();
	const col = collections.find((c) => c.id === activeCollection);
	if (col === undefined || col.is_default) {
		return; // the button is disabled in both cases, see renderCollections
	}
	if (!confirm(`delete ${col.name} and every color in it?`)) {
		return;
	}
	try {
		await call("DELETE", `/me/collections/${col.id}`);
		collections = collections.filter((c) => c.id !== col.id);
		setStatus(`deleted ${col.name}`);
		// the grid is showing a collection that no longer exists
		setActiveCollection(pickCollection(null));
		await loadSaved();
	} catch (err) {
		setStatus(err.message, true);
	}
}

// The other one: the collection pages, so what goes includes colors no grid on screen has. The
// collection itself stays, which is the whole difference between this and the delete above.
async function emptyCollection() {
	if (!requireSession()) {
		return;
	}

	// resolved before the question, so the name in it is the collection the request will hit
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
	await ensureCollections(); // the row goes onto a list, so there has to be one
	const { data } = await call("POST", "/me/collections", { name, icon, accent });
	collections.push(data.collection);
	setStatus(`${data.collection.name} created`);
	selectCollection(data.collection.id);
}

// ---- swatches ----

// The selection: one hex and one caption, either a palette name or a saved timestamp. Marks the
// swatch in both grids and fills the aside. Dropped as soon as the hex field says something else.
let selectedHex = null;
let selectedLabel = "";

// What the page opens on: the title's first tint, see the h1[data-tint="1"] rule. Not a palette
// row, so no swatch carries the mark until the first click. The label names the source, the way
// "picked" and "random" do.
const DEF_SELECTION = { hex: "#bf15a3", name: "title" };

// The selection survives a reload, so the page comes back on the color that was being looked at
// rather than on the title's tint. A hex that no longer parses falls through to DEF_SELECTION, the
// way a stored control outside CONTROL_VALUES does.
//
// A dropped selection is stored as null and opens on DEF_SELECTION too: restoring the empty state
// would put the three blank panels back, which is what DEF_SELECTION exists to avoid.
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

// The inputs that hold a hex: the two pickers and the add form's field, which is what "add" saves.
// They mirror the selection, so each lands on a color chosen anywhere else and can nudge it.
const HEX_INPUTS = ["pick", "add-pick", "hex"];

// A producer already holds the value, so it writes the others and not itself: a write into a
// picker while its dialog is open fights the dialog for the value, and a write into the field
// moves the caret. Passing no id writes all three, which is what a swatch or a button wants.
function writeHexInputs(hex, from) {
	for (const id of HEX_INPUTS) {
		if (id !== from) {
			$(id).value = hex;
		}
	}
}

// Swatches, the random button and a paste: none of them is one of the inputs, so all three follow.
function selectColor(hex, label) {
	writeHexInputs(hex);
	commitSelection(hex, label);
}

// one input's own path
function selectFrom(id, hex, label) {
	writeHexInputs(hex, id);
	commitSelection(hex, label);
}

// Every producer comes through here, so the inputs and the panels beside them cannot disagree.
function commitSelection(hex, label) {
	selectedHex = hex;
	selectedLabel = label;
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
	$("pick").disabled = selectedHex === null;

	// every producer ends here, so this is the one place the reload has to read back
	const stored = selectedHex === null ? null : { hex: selectedHex, name: selectedLabel };
	writeStored(SELECTION_KEY, stored);

	// a strip built for the previous color would be wrong, so every pinned one follows the
	// selection or empties with it
	showStrips();
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
// a browser policy can still refuse. A refusal breaks nothing, so it is only reported.
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

// The same toggle, from the button that was pressed. A press that ends full screen gives the page
// back at once, so the dip comes off before it plays: the button is under the pointer when the
// page returns, and a color dipping there reads as a swatch that was just picked. The delegated
// listener runs in the capture phase, so the class is already on the button here.
function toggleFullscreenFrom(button, el) {
	if (document.fullscreenElement !== null) {
		button.classList.remove("pressed");
	}
	toggleFullscreen(el);
}

// navigator.clipboard exists only in a secure context, so over plain http the property is missing
// and reading through it throws. In an async function that is a rejection like any other.
// The dip on a press is on every button, so a copy needs none of its own. The log line is what
// separates a copy that failed from one that went through.
async function copyHex(hex) {
	try {
		await navigator.clipboard.writeText(hex);
		setStatus(`copied ${hex}`);
	} catch (err) {
		setStatus(`could not copy - ${err.message}`, true);
	}
}

// The read is a permission in some browsers and the property is missing outside a secure context,
// which is a rejection like any other. A clipboard that is not a hex says so rather than landing
// in the field: the field would clear the selection, and the button would look like it did nothing.
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
	selectColor(hex, "pasted");
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

	// no text of its own, so the name is spelled out
	const block = document.createElement("button");
	block.type = "button";
	block.className = "detail-color";
	block.style.background = hex;
	block.title = "full screen, click again to leave";
	block.setAttribute("aria-label", `show ${hex} full screen`);
	block.addEventListener("click", () => toggleFullscreenFrom(block, block));

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

// ---- color pickers ----
// Three <input type="color">, and the dialog belongs to the browser in all of them. The one in the
// Selected panel nudges the selection and the one in the add row starts one, so what both produce
// is a selection, the same as a swatch click. The one in the Picker panel is a scratch pad and
// touches nothing else on the page.
//
// A drag reports every color it passes through, and with a harmony on screen each selection is a
// request. So the selection waits for the drag to go quiet.
const PICK_QUIET = 200;

let pickTimer = null;

// One timer for both: a pointer is in one dialog at a time, so the gesture that lands last is the
// one to commit.
function initPicker() {
	for (const id of ["pick", "add-pick"]) {
		// input reports each step of a drag, change the committed value. Which of them a browser
		// sends and how often varies, so both schedule the same commit and the timer collapses the
		// gesture.
		for (const type of ["input", "change"]) {
			$(id).addEventListener(type, () => {
				clearTimeout(pickTimer);
				pickTimer = setTimeout(() => selectFrom(id, $(id).value, "picked"), PICK_QUIET);
			});
		}
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

// The names the API derives from one color, in its own order out of color.HarmonyNames. One
// endpoint answers for all of them in one shape, so one loader draws any of them. Each name is the
// suffix of its pin's id and of the div its strip lands in.
const DERIVED = ["complement", "split-complement", "triad", "analogous", "square", "ramp", "tones"];

// The rotations lead their strip with the selection, since the pairing is the point. A scale is
// seven points on an axis and the selection is not one of them, so a band for it would sit at the
// head reading as a step out of order.
const LEADING = new Set(["complement", "split-complement", "triad", "analogous", "square"]);

// The axis a scale sweeps, which its name does not say. The pin and the strip's own label both
// carry it as a title.
const AXIS = {
	ramp: "lightness, dark to light",
	tones: "chroma, the gray of this lightness to the full color",
};

// What the page opens on, which is what it showed before the pins: one rotation and both scales.
const DEF_PINNED = ["complement", "ramp", "tones"];

// Each strip on screen keeps the bands it was built from, by name: { hex, bands }. The hex is what
// lets a pin come back without a request, and what keeps an older selection's colors off the page
// and out of an add.
const stripColors = {};

// Same guard as loadSaved, one number for the whole section: the pins stay live while a request is
// out, so an earlier response landing later must not replace a fresher strip.
let stripGeneration = 0;

// A closed section asks for nothing. What it missed while closed is what it loads when it opens.
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

// A block per pinned name, built here rather than declared in the markup: which of them are on is
// the user's. Every block is the same label, buttons and strip, so a scale is built like a
// rotation.
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

// A pin whose bands were derived from the color already selected is drawn from memory: the request
// would answer what stripColors holds.
function showStrips() {
	if (!$("harmony-section").open) {
		stripsStale = true;
		return;
	}
	stripsStale = false;

	const generation = ++stripGeneration;
	for (const name of pinned) {
		if (selectedHex === null) {
			delete stripColors[name];
			drawStrip(name, null);
		} else if (stripColors[name]?.hex === selectedHex) {
			drawStrip(name, stripColors[name].bands);
		} else {
			loadStrip(name, selectedHex, generation);
		}
	}
}

async function loadStrip(name, hex, generation) {
	try {
		// bare six digits in the path, same rule as the delete above
		const { data } = await call("GET", `/colors/${hex.slice(1)}/${name}`);
		if (generation !== stripGeneration) {
			return; // a later selection owns the section now
		}
		// the answer is the normalized hex asked about, the name, and the colors it derives
		const bands = LEADING.has(name) ? [data.hex, ...data.colors] : data.colors;
		stripColors[name] = { hex, bands };
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

// A band selects, the way a swatch does, so a step can be picked up and worked on. The block's own
// button is what sends the strip full screen. A strip already on the page is refilled rather than
// rebuilt, see fillStrip.
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

// The strip is already on the page, so it goes full screen where it stands. The saved grid has to
// build one first, since a grid of cells is not a strip.
function showStripFullscreen(name) {
	const strip = $(`strip-${name}`)?.querySelector(".harmony") ?? null;
	if (strip === null) {
		setStatus("nothing to show", true);
		return;
	}
	toggleFullscreen(strip);
}

// The bands on screen, the leading selection included: the button sits over the strip, so what it
// adds is what the strip shows. A band the collection already holds answers 200 rather than 201, so
// created counts what was new, the same shape the bulk run reports.
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

// The saved grid as the same strip, straight to full screen. It is the swatches on screen, pages
// loaded so far in their sort order, so what goes full screen is what the grid shows.
function showSavedFullscreen() {
	const hexes = [...document.querySelectorAll("#saved .swatch")].map((el) => el.dataset.hex);
	if (hexes.length === 0) {
		setStatus("nothing to show", true);
		return;
	}
	showOffstage(buildStrip(hexes));
}

// The palette full screen: the grid on screen, cloned. A strip of bands has nowhere to put a
// name, and the name is what a palette cell is for. The clone drops the listeners with it, so a
// swatch there selects nothing and a click only leaves, the way a band does.
//
// Dense whatever the toggle says, unlike the saved strip, which is the swatches on screen. The
// roomy track is 9rem cells against a 49.5rem column, and a screen takes eight of those across:
// 100 colors then want thirteen rows, and a row is too short for the two lines a cell holds. Ten
// across is the count that puts the whole set on one screen with room for the names.
function showPaletteFullscreen() {
	const grid = $("palette");
	if (grid.querySelector(".swatch") === null) {
		setStatus("nothing to show", true);
		return;
	}
	const copy = grid.cloneNode(true);
	copy.classList.add("dense");
	copy.removeAttribute("id"); // two of an id, and $("palette") could answer with this one

	// Inert, like the listeners the clone dropped. "use #xxxxxx" names something a click there
	// does not do, and the selection is a mark on the page, not on a screen of colors - without
	// the hex a later markSelected cannot put the ring back either.
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
// gone when it comes down. Neither the saved strip nor the palette copy has a block of its own.
async function showOffstage(el) {
	el.classList.add("offstage");
	document.body.append(el);

	// leaving full screen, by Escape or by a click inside, is when the element goes
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

// A band click toggles full screen by default, which is what the saved grid's off-stage strip
// needs: it has no block to carry a button. A caller that passes onBand takes the click instead
// and carries its own button, see drawStrip.
function buildStrip(hexes, onBand) {
	const strip = document.createElement("div");
	strip.className = "harmony";
	fillStrip(strip, hexes, onBand);
	return strip;
}

/*
The colors of a strip that is already up, onto the bands it already has. A band clicked to select
reloads every pinned strip, and a rebuilt one takes that band off the page while its press dip is
still playing, so the band count is matched and the rest is written over what is there. A strip
keeps its count across a selection (a triad stays three), so the loops below are the edges: a
first fill, and the saved grid's strip, which is a page of swatches and can be any length.
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
		// no title: this strip is built off stage and goes straight up, so the tip would only
		// ever be read over a screen the click leaves
		block.addEventListener("click", () => toggleFullscreenFrom(block, strip));
	} else {
		block.addEventListener("click", () => {
			// full screen is the strip itself, so a band in it only leaves: the selection stays
			// where it was. A refill cannot end it the way a rebuilt strip did, see fillStrip.
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
			data.colors.map((c) => swatch(c.hex, c.name)),
		);
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
// Dev only. There is no bulk endpoint: these are ordinary POSTs, a few in flight at a time. All at
// once is a burst nothing else on this page produces, one at a time is a round trip each.

const BULK_COUNT = 10;
const BULK_IN_FLIGHT = 8;

async function addRandomColors() {
	if (!requireSession()) {
		return;
	}

	// Resolved once, before any worker starts. A failed lookup ends the run here rather than
	// failing every add with the same message.
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
// the dialog only opens logged out.
const ACCOUNT_TABS = ["login", "signup"];

// aria-selected is the record, and the CSS reads it: nothing else tracks which tab is up. With the
// dialog open the focus follows into the form, since the tab was clicked to type there.
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
	$("logged-out").hidden = session !== null;
	$("logged-in").hidden = session === null;
	$("saved-section").hidden = session === null;

	if (session === null) {
		// the cursor was minted for the session that just ended and carries no user of its own,
		// so a later load-more would append the previous account's next page
		setNextCursor(null);
		// the grid still holds that account's swatches, and the next login unhides this section
		// before its own request lands
		renderEmpty($("saved"), "nothing saved");
		return;
	}
	// empty while GET /me is in flight, and after it failed
	$("who").textContent = session.nickname || "logged in";
}

// Two requests: the token, then the account it belongs to. GET /me is authenticated, so the session
// has to exist before the stored nickname can be asked for, and the panel renders nameless until it
// lands.
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
	$("theme").title = `${next} theme`;
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

function applyZen(on) {
	document.documentElement.classList.toggle("zen", on);
	// the glyph names what a click does, as the two words it replaced did
	$("zen").replaceChildren(spriteSvg(on ? "ui-eye" : "ui-eye-off", "icon"));
	$("zen").title = "zen mode (z)";
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

// Every listing control and the values the API accepts, mirroring the markup. Anything not listed
// is dropped when a stored preference is read, so a stale key cannot produce a 400. The order here
// is the order a cycler steps through.
const CONTROL_VALUES = {
	sort: ["created_at", "hex", "color"],
	order: ["desc", "asc"],
	limit: ["10", "20", "50", "100"],
	"palette-sort": ["name", "hex", "color"],
	"palette-order": ["asc", "desc"],
};

// A label per value for the controls that are buttons rather than menus. Being listed here is what
// makes a control a cycler, see isCycle below.
const CYCLE_LABELS = {
	sort: { created_at: "date", hex: "hex", color: "color" },
	order: { desc: "desc \u2193", asc: "asc \u2191" },
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

$("theme").addEventListener("click", () => {
	applyTheme(currentTheme() === "dark" ? "light" : "dark");
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
	or a nickname is not a shortcut, and while a dialog is up, where the page behind the backdrop
	is not what a key should reach. A modifier means the key belongs to the browser.
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

// A complete hex, typed or pasted, is a selection. Anything else clears it: a mark that does not
// match the field would name a color other than the one about to be added.
$("hex").addEventListener("input", () => {
	const hex = parseHex($("hex").value);
	if (hex === null) {
		if (selectedHex !== null) {
			clearSelection();
		}
	} else if (hex !== selectedHex) {
		selectFrom("hex", hex, "typed");
	}
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

// a selection like any other, so the panels show what the button produced
$("random-hex").addEventListener("click", () => {
	selectColor("#" + randomDigits(), "random");
});

$("paste-hex").addEventListener("click", pasteHex);

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

// the button, not the row around it: a click beside it is not a click on the control
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

// The nickname has to be in hand before the dialog opens, since it is what the field is checked
// against, so a session stored before the nickname was kept spends a GET /me here.
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
		await createCollection($("collection-name").value, selectedIcon, $("collection-accent").value);
		$("collection-name").value = "";
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
		$("collection-name").focus();
	}
}

$("collection-new").addEventListener("click", () => {
	openCollectionForm($("collection-new").getAttribute("aria-expanded") !== "true");
});

$("collection-empty").addEventListener("click", emptyCollection);
$("collection-delete").addEventListener("click", deleteCollection);

for (const grid of Object.keys(DENSE_GRIDS)) {
	$(`${grid}-dense`).addEventListener("click", () => applyDense(grid, !denseOn(grid)));
}

$("saved-fullscreen").addEventListener("click", showSavedFullscreen);
$("palette-fullscreen").addEventListener("click", showPaletteFullscreen);

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

renderThemeButton(currentTheme());
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
renderCollections(); // the empty list, until the first response replaces it
renderSession(); // picks the account panel, and hides the two sections that need a token
loadPalette(); // public, so it runs logged out too
if (session !== null) {
	loadSaved(); // pulls the collection list on its way, see savedColorsPath
}
