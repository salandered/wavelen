// The page's pure helpers: values in, values out, no DOM and no network. Covered by lib.test.js.

// The server's own normalization, see color.ParseHex: trimmed, the '#' optional, case folded.
// Null is a field that names no color yet.
export function parseHex(text) {
	const digits = text.trim().replace(/^#/, "");
	return /^[0-9a-fA-F]{6}$/.test(digits) ? `#${digits.toLowerCase()}` : null;
}

// Perceived brightness (the YIQ weights) picks between a black and a white label. Not a contrast
// ratio, but one line and enough to keep every swatch readable.
export function labelColor(hex) {
	const n = parseInt(hex.slice(1), 16);
	const r = (n >> 16) & 0xff;
	const g = (n >> 8) & 0xff;
	const b = n & 0xff;
	return (r * 299 + g * 587 + b * 114) / 1000 > 140 ? "#000" : "#fff";
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
export function savedLabel(at) {
	return savedTime
		.formatToParts(at)
		.map((part) => (part.type === "literal" ? part.value.replace(",", "") : part.value))
		.join("");
}

// For poking at the API by hand. Math.random is enough: nothing here is a secret, and a collision
// is a color the account already saved, which answers 200 instead of 201.
export function randomDigits() {
	return Math.floor(Math.random() * 0x1000000)
		.toString(16)
		.padStart(6, "0");
}

// The download name comes from the response's Content-Disposition, so the page keeps no second
// copy of the server's format. Only the quoted form is parsed, which is what the API sends.
export function exportFilename(disposition) {
	return /filename="([^"]+)"/.exec(disposition ?? "")?.[1] ?? "wavelen-export.json";
}
