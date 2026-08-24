// Everything the page knows about the service is in api.yaml. Same origin, caddy proxies
// /api/v1/* to the app, so no CORS header exists anywhere.
const API = "/api/v1";

// "theme" is written by the inline script in <head> and stays a raw string. Everything read
// through readStored below is JSON, so the two never share a key.
const DETAILS_KEY = "details_open";
const SWATCH_INFO_KEY = "swatch_info";

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

function writeStored(key, value) {
	try {
		localStorage.setItem(key, JSON.stringify(value));
	} catch {
		// nothing to do, the preference just does not survive the reload
	}
}

// ---- requests ----

// Errors come back as {"error": "..."}, but a proxy or a panic can still produce a non-JSON
// body with an error status, so the body is read as text and parsed opportunistically.
async function call(method, path, body) {
	const options = { method };
	if (body !== undefined) {
		options.headers = { "Content-Type": "application/json" };
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
	if (!res.ok) {
		throw new Error(`${res.status} - ${data?.error ?? text}`);
	}
	return { status: res.status, data };
}

function setStatus(message, failed = false) {
	$("status").textContent = message;
	$("status").classList.toggle("failed", failed);
}

function userID() {
	const id = $("user-id").value.trim();
	if (id === "") {
		setStatus("enter a user id", true);
		return null;
	}
	return id;
}

// ---- swatches ----

// The hex the last clicked swatch put in the field. Marks that swatch in both grids, and is
// dropped as soon as the field says something else.
let selectedHex = null;

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
	el.addEventListener("click", () => {
		selectedHex = hex;
		$("hex").value = hex;
		markSelected();
	});
	return el;
}

function renderEmpty(container, message) {
	const p = document.createElement("p");
	p.className = "empty";
	p.textContent = message;
	container.replaceChildren(p);
}

// A swatch keeps its mark across a re-render, so adding a color does not clear it.
function renderSwatches(container, swatches) {
	container.replaceChildren(...swatches);
	markSelected();
}

// ---- data ----

async function loadPalette() {
	try {
		const { data } = await call("GET", "/colors");
		renderSwatches($("palette"), data.colors.map((c) => swatch(c.hex, c.name)));
	} catch (err) {
		renderEmpty($("palette"), err.message);
	}
}

// An unknown user answers 200 with an empty array, same as one who saved nothing, so an empty
// grid here does not mean the user exists.
async function loadSaved() {
	const id = userID();
	if (id === null) {
		return;
	}
	try {
		const { data } = await call("GET", `/users/${id}/colors`);
		if (data.colors.length === 0) {
			renderEmpty($("saved"), "nothing saved");
			return;
		}
		const swatches = data.colors.map((c) =>
			swatch(c.hex, new Date(c.created_at).toLocaleString()));
		renderSwatches($("saved"), swatches);
	} catch (err) {
		renderEmpty($("saved"), "");
		setStatus(err.message, true);
	}
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

// app.js is deferred, so the document is complete here. A section with no stored preference
// keeps whatever `open` its markup declares.
function initCollapsibleSections() {
	const stored = readStored(DETAILS_KEY, {});
	const state = stored !== null && typeof stored === "object" && !Array.isArray(stored) ? stored : {};
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
		selectedHex = null;
		markSelected();
	}
});

$("load").addEventListener("click", () => {
	setStatus("");
	loadSaved();
});

$("create-user").addEventListener("click", async () => {
	try {
		const { data } = await call("POST", "/users", {
			email: $("new-email").value,
			name: $("new-name").value,
		});
		$("user-id").value = data.user.id;
		$("new-email").value = "";
		$("new-name").value = "";
		setStatus(`user ${data.user.id} created`);
		await loadSaved();
	} catch (err) {
		setStatus(err.message, true);
	}
});

// Adding is idempotent: 201 means it was new, 200 means the user already had it.
$("add-form").addEventListener("submit", async (event) => {
	event.preventDefault();
	const id = userID();
	if (id === null) {
		return;
	}
	try {
		const { status, data } = await call("POST", `/users/${id}/colors`, { hex: $("hex").value });
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

loadPalette();
loadSaved();
