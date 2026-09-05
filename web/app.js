// Everything the page knows about the service is in api.yaml. Same origin, caddy proxies
// /api/v1/* to the app, so no CORS header exists anywhere.
const API = "/api/v1";

// "theme" is written by the inline script in <head> and stays a raw string. Everything read
// through readStored below is JSON, so the two never share a key.
const DETAILS_KEY = "details_open";
const SWATCH_INFO_KEY = "swatch_info";
const SESSION_KEY = "session";
const CONTROLS_KEY = "controls";

const $ = (id) => document.getElementById(id);

// ---- browser storage ----
// Persistence is optional everywhere: a private window or blocked site data leaves the page
// working, just without preferences.

function readStored(key, fallback) {
	try {
		const raw = localStorage.getItem(key);
		return raw === null ? fallback : JSON.parse(raw);
	} catch {
		return fallback;
	}
}

// A stored JSON object, or {} for anything else in the key. Two callers keep a map under one
// key, and both would rather ignore a hand-edited value than run into it later.
function readStoredObject(key) {
	const stored = readStored(key, null);
	const usable = stored !== null && typeof stored === "object" && !Array.isArray(stored);
	return usable ? stored : {};
}

function writeStored(key, value) {
	try {
		localStorage.setItem(key, JSON.stringify(value));
	} catch {
		// nothing to do, the preference just does not survive the reload
	}
}

// ---- session ----
// What POST /api/v1/tokens answered - the bearer token and when it stops working - plus the name
// GET /me answered with, which is the label on the account panel. The name is stored rather than
// re-fetched so a reload renders the panel without a request. It is a caption: a stale one is
// wrong on the screen and nowhere else.
//
// This sits in localStorage, which any script on the origin can read. The page loads no
// third-party script and builds every node with textContent, so there is nothing here to read it.
// A cookie would need the server to set it and would need CSRF handling in exchange for hiding
// the token from a script that does not exist. Revisit if that stops being true.

let session = null;

// A stored session that has expired is dropped rather than sent. The server would refuse it, this
// only saves the doomed request. Nothing here is a security check, the 401 below is.
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
	writeStored(SESSION_KEY, session);
	renderSession();
}

// The name arrives one request after the token, see login().
function setSessionName(name) {
	session = { ...session, name };
	writeStored(SESSION_KEY, session);
	renderSession();
}

// Local only. Whether the token is dead server-side is the caller's business: logout revokes it
// first, an expiry or a 401 means it already is.
function endSession() {
	session = null;
	writeStored(SESSION_KEY, null);
	renderSession();
}

// ---- requests ----

// Errors come back as {"error": "..."}, but a proxy or a panic can still produce a non-JSON
// body with an error status, so the body is read as text and parsed opportunistically.
//
// The token goes on every request once there is one, public paths included. Those ignore it, and
// one rule beats a flag at each call site that only has to be forgotten once to send nothing.
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

	// There is no refresh, so a 401 with a token in hand means that token is finished: expired,
	// revoked, or revoked from another tab. The page goes back to logged out rather than leaving
	// controls that answer 401 on every click.
	if (res.status === 401 && session !== null) {
		endSession();
	}
	if (!res.ok) {
		throw new Error(`${res.status} - ${data?.error ?? text}`);
	}
	return { status: res.status, data };
}

// ---- log ----

// The last few things that happened, newest first. In memory only: it says what this page just
// did, which is not worth a sixth localStorage key and would be a lie after a reload.
const LOG_LIMIT = 10;
const logEntries = [];

// A transient entry is a live counter - the bulk run rewrites one line rather than filling the
// panel with its own progress - so the next entry replaces it instead of landing under it. That
// makes the line that finishes a run read as the one the run left behind.
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

// The saved-colors calls need a token, not an id: /api/v1/me/colors resolves the user server-side.
// This only stops a logged out click from sending a request that could only answer 401.
function requireSession() {
	if (session === null) {
		setStatus("log in first", true);
		return false;
	}
	return true;
}

// A saved swatch is captioned with its timestamp, and the label is one line that must fit a
// swatch: a 2-digit year keeps the whole stamp on it, where toLocaleString's 4-digit one does
// not. Locale order is still the browser's, this only shortens the year.
const savedTime = new Intl.DateTimeFormat(undefined, {
	year: "2-digit",
	month: "2-digit",
	day: "2-digit",
	hour: "2-digit",
	minute: "2-digit",
	second: "2-digit",
});

// The comma most locales put between the date and the time is the one separator this caption does
// not need - a space already reads as the break. Dropped through formatToParts rather than off the
// formatted string, so only the format's own literals are touched and the locale keeps its order.
function savedLabel(at) {
	return savedTime
		.formatToParts(at)
		.map((part) => (part.type === "literal" ? part.value.replace(",", "") : part.value))
		.join("");
}

// ---- swatches ----

// The hex the last clicked swatch put in the field, and the caption that swatch carried: a palette
// name or a saved timestamp. Marks the swatch in both grids and fills the panel beside them, and
// both are dropped as soon as the field says something else.
let selectedHex = null;
let selectedLabel = "";

// Perceived brightness (the YIQ weights) decides between a black and a white label. It is not
// a contrast ratio, but it is one line and enough to keep every swatch readable.
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

// Clicking a swatch fills the hex field, which is the only thing the palette is for.
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
	el.addEventListener("click", () => selectSwatch(hex, label));
	return el;
}

function selectSwatch(hex, label) {
	selectedHex = hex;
	selectedLabel = label;
	$("hex").value = hex;
	selectionChanged();
}

function clearSelection() {
	selectedHex = null;
	selectedLabel = "";
	selectionChanged();
}

// Both aside panels are about the one selected color, so nothing may move only one of them.
function selectionChanged() {
	markSelected();
	renderDetail();
	$("complement").disabled = selectedHex === null;
	$("triad").disabled = selectedHex === null;

	// A strip built for the previous color would be a wrong answer sitting on the page, so the
	// harmony either follows the selection or goes back to its placeholder.
	if (harmonyMode !== null && selectedHex !== null) {
		showHarmony(harmonyMode);
	} else {
		renderHarmonyPlaceholder();
	}
}

// A saved swatch carries a delete control, the palette's does not: the palette is read-only.
// Siblings inside a cell, not one inside the other, same as the detail stack - a button cannot
// contain a button. The cell is what the grid lays out and what the hover rule keys off.
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
// The cell is dropped rather than the list reloaded, because a reload would drop every appended
// page and send the user back to the first one. The cursor survives a delete on its own: it is a
// value compared against, not a reference to the row it was minted from.
async function deleteColor(hex, cell, button) {
	if (!requireSession()) {
		return;
	}
	button.disabled = true; // a second click while the first is out would answer 404
	try {
		await call("DELETE", `/me/colors/${hex.slice(1)}`);
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

// The field is the only input to the add form, so anything writing it programmatically has to
// drop the mark too: an input event does not fire for a scripted value.
function setHexField(hex) {
	$("hex").value = hex;
	clearSelection();
}

function renderEmpty(container, message) {
	const p = document.createElement("p");
	p.className = "empty";
	p.textContent = message;
	container.replaceChildren(p);
}

// The Fullscreen API needs a user gesture, which the click is, but it can still be refused by an
// iframe without allowfullscreen or by a browser policy. Nothing is broken when it is, so the
// refusal only has to be visible.
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
// and reading through it throws. Inside an async function that is a rejection like any other.
async function copyHex(hex) {
	try {
		await navigator.clipboard.writeText(hex);
		setStatus(`copied ${hex}`);
	} catch (err) {
		setStatus(`could not copy - ${err.message}`, true);
	}
}

// The panel beside the grids. One panel for both of them, since the selection they share is one
// hex: clicking in either grid replaces what this shows.
function renderDetail() {
	if (selectedHex === null) {
		renderEmpty($("detail"), "click a color");
		return;
	}
	const hex = selectedHex; // captured, so a later selection does not rewrite these handlers

	// Siblings, not one inside the other: a button cannot contain a button, and stacking them
	// means neither click has to be kept from reaching the other.
	const stack = document.createElement("div");
	stack.className = "detail-stack";

	// no text of its own, so the color is the whole control and the name has to be spelled out
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

// ---- harmony ----

// Which harmony the panel is showing, so clicking a different swatch answers with the same one
// for the new color instead of making the button be pressed again.
let harmonyMode = null;

// Same guard as loadSaved: the buttons stay live while a request is out, so an earlier response
// landing later must not replace a fresher strip.
let harmonyGeneration = 0;

// The endpoint answers with the color asked about beside the others, and the strip shows all of
// them: the point is the pairing, and a complement on its own is not one.
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

// The strip is what goes full screen, not a band: the harmony is the thing worth looking at, and
// one color of it full screen is what the Selected panel already does. Each band is a stack of
// two controls for the usual reason - a button cannot contain a button - so the strip itself is a
// plain element and every band carries its own pair.
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

// A swatch keeps its mark across a re-render, so adding a color does not clear it.
function renderSwatches(container, swatches) {
	container.replaceChildren(...swatches);
	markSelected();
}

// Keeps the pages already shown. markSelected has to run again either way: these swatches were
// built after the last call and carry no mark of their own.
function appendSwatches(container, swatches) {
	container.append(...swatches);
	markSelected();
}

// ---- data ----

// Sorting by hex is lexicographic, so it groups by the red channel rather than by anything
// perceptual. It is still the useful second view: names and hex codes disagree completely.
async function loadPalette() {
	const params = new URLSearchParams({
		sort: $("palette-sort").value,
		order: $("palette-order").dataset.order,
	});
	try {
		const { data } = await call("GET", `/colors?${params}`);
		renderSwatches($("palette"), data.colors.map((c) => swatch(c.hex, c.name)));
	} catch (err) {
		renderEmpty($("palette"), err.message);
	}
}

// The cursor for the next page of the saved list, or null when there is no next page. It carries
// the sort and the order it was minted under, so the API itself rejects it once either of those
// changes. It carries no user, so nothing but this page can notice the user id moving: a fresh
// load drops the cursor, and so does editing the field.
let nextCursor = null;

// loadSaved can be in flight more than once, since the controls stay live while a request is out.
// Each call takes the next number and only touches the grid while its number is still the current
// one, so a request that started earlier and landed later cannot overwrite a fresher one.
let savedGeneration = 0;

// The button is the only way to page, so hiding it is the end-of-list signal made visible.
function setNextCursor(cursor) {
	nextCursor = cursor;
	$("load-more").hidden = cursor === null;
}

function savedQuery(cursor) {
	const params = new URLSearchParams({
		sort: $("sort").value,
		order: $("order").dataset.order,
		limit: $("limit").value,
	});
	if (cursor !== null) {
		params.set("cursor", cursor);
	}
	return params;
}

// An empty grid means the token's user saved nothing, or that the cursor ran off the end. Both
// answer 200 with an empty array and there is no third case: the token proves the user exists.
async function loadSaved({ append = false } = {}) {
	if (!requireSession()) {
		return;
	}
	const cursor = append ? nextCursor : null;
	const generation = ++savedGeneration;

	// the cursor stays in nextCursor until the response replaces it, so a second click while this
	// one is out would send it again and append the same page twice
	$("load-more").disabled = true;
	try {
		const { data } = await call("GET", `/me/colors?${savedQuery(cursor)}`);
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
		// only the newest request re-enables it, so it stays disabled while another one is still out
		if (generation === savedGeneration) {
			$("load-more").disabled = false;
		}
	}
}

// ---- random values ----
// For poking at the API by hand. Math.random is enough: nothing here is a secret, and the only
// collision that matters is a taken nickname, which answers 409 and asks again.

// Six hex digits. A color is these with a "#" in front; a user takes them as a tag shared by
// the nickname and the name, so the two read as one user.
function randomDigits() {
	return Math.floor(Math.random() * 0x1000000).toString(16).padStart(6, "0");
}

// ---- bulk add ----
// Dev only. There is no bulk endpoint and this does not ask for one: it is ordinary POSTs, a few
// in flight at a time. All of them at once is a burst nothing else on this page produces, and one
// at a time is a round trip each.

const BULK_COUNT = 20;
const BULK_IN_FLIGHT = 8;

async function addRandomColors() {
	if (!requireSession()) {
		return;
	}

	// a set, so the run is BULK_COUNT distinct colors rather than that many draws with collisions
	const queue = new Set();
	while (queue.size < BULK_COUNT) {
		queue.add("#" + randomDigits());
	}
	const hexes = [...queue];

	let done = 0;
	let created = 0;
	let failed = 0;
	let firstError = null;

	// Each worker drains the same array, so a slow request does not hold up the others.
	const worker = async () => {
		while (hexes.length > 0) {
			const hex = hexes.pop();
			try {
				const { status } = await call("POST", "/me/colors", { hex });
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

// The saved list is the only part of the page that needs a token. The palette and the two
// harmonies are public, so a logged out visitor keeps a working page rather than an empty one.
function renderSession() {
	$("logged-out").hidden = session !== null;
	$("logged-in").hidden = session === null;
	$("saved-section").hidden = session === null;

	if (session === null) {
		// The cursor was minted for the session that just ended and carries no user of its own,
		// so a later load-more would append the previous account's next page.
		setNextCursor(null);
		// And the grid still holds that account's swatches. The next login unhides this section
		// before its own request lands, which would show one user their predecessor's colors.
		renderEmpty($("saved"), "nothing saved");
		return;
	}
	// empty while GET /me is in flight, and after it failed
	$("who").textContent = session.name || "logged in";
}

// Two requests: the token, then the account it belongs to. GET /me is authenticated, so the
// session has to exist before the name can be asked for, and the panel renders nameless until it
// lands. Logging in is the token; the name is a label on it.
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
		// the preference just does not survive a reload
	}
}

function applySwatchInfo(hidden) {
	document.documentElement.classList.toggle("no-swatch-info", hidden);
	$("swatch-info").textContent = hidden ? "show info" : "hide info";
	writeStored(SWATCH_INFO_KEY, hidden);
}

// Two values, so a button beats a menu: one click is the whole choice. The order itself lives in
// data-order, which is where the query builders read it; the label only reports it.
function initOrderToggle(id, onChange) {
	const el = $(id);
	const render = () => {
		const desc = el.dataset.order === "desc";
		el.textContent = desc ? "desc \u2193" : "asc \u2191";
		el.title = `switch to ${desc ? "ascending" : "descending"}`;
	};
	el.addEventListener("click", () => {
		el.dataset.order = el.dataset.order === "desc" ? "asc" : "desc";
		render();
		onChange();
	});
	render();
}

// Every listing control and the values it accepts, mirroring the options in the markup. A stale
// or hand-edited key must not put the page in a state the API answers with a 400, so anything not
// listed here is dropped and the markup's own value stands.
const CONTROL_VALUES = {
	"sort": ["created_at", "hex", "color"],
	"order": ["desc", "asc"],
	"limit": ["10", "20", "50", "100"],
	"palette-sort": ["name", "hex", "color"],
	"palette-order": ["asc", "desc"],
};

// The two toggles keep their value in data-order and everything else in .value. That is the only
// difference between the controls, and these three functions are where it lives.
function isOrderToggle(id) {
	return id === "order" || id === "palette-order";
}

function readControl(id) {
	return isOrderToggle(id) ? $(id).dataset.order : $(id).value;
}

function writeControl(id, value) {
	if (isOrderToggle(id)) {
		$(id).dataset.order = value;
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

// app.js is deferred, so the document is complete here. A section with no stored preference
// keeps whatever `open` its markup declares.
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

// A mark that no longer matches the field would lie about what is about to be added.
$("hex").addEventListener("input", () => {
	if (selectedHex !== null && $("hex").value !== selectedHex) {
		clearSelection();
	}
});

for (const mode of Object.keys(HARMONIES)) {
	$(mode).addEventListener("click", () => showHarmony(mode));
}

$("bulk-add").addEventListener("click", addRandomColors);

$("random-hex").addEventListener("click", () => {
	setHexField("#" + randomDigits());
});

// Fills the two fields and stops there. Creating is still the create button, so a generated
// user can be edited or discarded before it reaches the API.
$("random-user").addEventListener("click", () => {
	const tag = randomDigits();
	$("new-nick").value = `user-${tag}`;
	$("new-name").value = `user ${tag}`;
	$("new-password").value = `password-${tag}`; // dev only, and long enough for the API
});

$("load").addEventListener("click", () => {
	loadSaved();
});

$("load-more").addEventListener("click", () => {
	loadSaved({ append: true });
});

// Changing either of these invalidates the cursor, and loadSaved without append drops it. The
// order toggle does the same through initOrderToggle, which fires on click rather than change.
for (const control of [$("sort"), $("limit")]) {
	control.addEventListener("change", () => {
		rememberControls();
		loadSaved();
	});
}

$("palette-sort").addEventListener("change", () => {
	rememberControls();
	loadPalette();
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

// Signing up answers with the user and no token, so the password is spent twice: once to create
// the account and once to log into it. Minting a token stays the one endpoint that does it.
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

// The endpoint revokes this token and leaves the account's other tokens alone. Dropping the local
// copy is not enough on its own: the token would keep working until it expired, which on a shared
// machine is the case logging out exists for.
//
// The session ends either way. A failure here means the token may still be live, which is worth
// reporting, but keeping the page logged in with a token the user asked to be rid of is worse.
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

// Adding is idempotent: 201 means it was new, 200 means the user already had it.
$("add-form").addEventListener("submit", async (event) => {
	event.preventDefault();
	if (!requireSession()) {
		return;
	}
	try {
		const { status, data } = await call("POST", "/me/colors", { hex: $("hex").value });
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
initControls(); // before the toggles below, which label themselves from data-order
initOrderToggle("order", () => {
	rememberControls();
	loadSaved();
});
initOrderToggle("palette-order", () => {
	rememberControls();
	loadPalette();
});

renderDetail(); // the placeholder, until a swatch is clicked
renderHarmonyPlaceholder(); // same, and the buttons stay disabled until then
renderLog(); // same, until something happens
renderSession(); // shows one of the two account panels and hides the saved list without a token
loadPalette(); // public, so it runs logged out too
if (session !== null) {
	loadSaved();
}
