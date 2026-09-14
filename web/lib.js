// The page's simple helpers, no DOM or network.

// The server's own normalization, see color.NewHex.
// Null is a field that names no color yet.
export function parseHex(text) {
	const digits = text.trim().replace(/^#/, "");
	return /^[0-9a-fA-F]{6}$/.test(digits) ? `#${digits.toLowerCase()}` : null;
}

// Perceived brightness (the YIQ weights) picks between a black and a white label.
export function labelColor(hex) {
	const n = parseInt(hex.slice(1), 16);
	const r = (n >> 16) & 0xff;
	const g = (n >> 8) & 0xff;
	const b = n & 0xff;
	return (r * 299 + g * 587 + b * 114) / 1000 > 140 ? "#000" : "#fff";
}

const savedTime = new Intl.DateTimeFormat(undefined, {
	year: "2-digit",
	month: "2-digit",
	day: "2-digit",
	hour: "2-digit",
	minute: "2-digit",
	second: "2-digit",
});

// Comma between date and time is dropped.
export function savedLabel(at) {
	return savedTime
		.formatToParts(at)
		.map((part) => (part.type === "literal" ? part.value.replace(",", "") : part.value))
		.join("");
}

// Math.random is ok: nothing here is a secret; a collision
// would simply answer 200.
export function randomDigits() {
	return Math.floor(Math.random() * 0x1000000)
		.toString(16)
		.padStart(6, "0");
}

// Pre-typed collection names.
const COLLECTION_NAMES = [
	"ash",
	"amber",
	"bark",
	"clay",
	"coral",
	"dusk",
	"ember",
	"fern",
	"indigo",
	"linen",
	"mint",
	"moss",
	"ochre",
	"plum",
	"rust",
	"sand",
	"slate",
	"umber",
];

export function randomCollectionName() {
	return COLLECTION_NAMES[Math.floor(Math.random() * COLLECTION_NAMES.length)];
}

// Only the quoted form is parsed - what the API sends.
export function exportFilename(disposition) {
	return /filename="([^"]+)"/.exec(disposition ?? "")?.[1] ?? "wavelen-export.json";
}
