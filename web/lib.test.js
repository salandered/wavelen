import { test } from "node:test";
import assert from "node:assert/strict";

import { exportFilename, labelColor, parseHex, randomDigits, savedLabel } from "./lib.js";

test("parseHex accepts optional hash and folds case", () => {
	assert.equal(parseHex("#FF00AA"), "#ff00aa");
	assert.equal(parseHex("ff00aa"), "#ff00aa");
	assert.equal(parseHex("  #Ff00Aa  "), "#ff00aa");
});

test("parseHex rejects anything but six hex digits", () => {
	assert.equal(parseHex(""), null);
	assert.equal(parseHex("#"), null);
	assert.equal(parseHex("ff00a"), null);
	assert.equal(parseHex("ff00aaa"), null);
	assert.equal(parseHex("gg00aa"), null);
	assert.equal(parseHex("#ff 00 aa"), null);
});

test("parseHex drops one leading hash, not two", () => {
	assert.equal(parseHex("##ff00aa"), null);
});

test("labelColor answers black on light and white on dark", () => {
	assert.equal(labelColor("#ffffff"), "#000");
	assert.equal(labelColor("#000000"), "#fff");
});

test("labelColor weights green over red over blue", () => {
	// same digit in each channel, and only the green one clears the threshold
	assert.equal(labelColor("#00ff00"), "#000");
	assert.equal(labelColor("#ff0000"), "#fff");
	assert.equal(labelColor("#0000ff"), "#fff");
});

test("labelColor switches at threshold 140", () => {
	// gray, so brightness is the channel value: 140 is not over it, 141 is
	assert.equal(labelColor("#8c8c8c"), "#fff"); // 0x8c = 140
	assert.equal(labelColor("#8d8d8d"), "#000"); // 0x8d = 141
});

test("savedLabel keeps year 2-digit and drops comma", () => {
	const label = savedLabel(new Date(2026, 8, 12, 4, 5, 6));

	assert.ok(!label.includes(","), `comma left in ${label}`);
	assert.ok(!label.includes("2026"), `4-digit year in ${label}`);
	assert.ok(label.includes("26"), `no 2-digit year in ${label}`);
});

// t.mock is restored when the test ends, a failed assertion included. A mock on the shared
// tracker would stay in place for the tests after it.
test("randomDigits pads short value to six digits", (t) => {
	t.mock.method(Math, "random", () => 0);
	assert.equal(randomDigits(), "000000");
});

test("randomDigits stays inside six digits at top of range", (t) => {
	// the top of what Math.random can answer, so 0x1000000 itself is out of reach
	t.mock.method(Math, "random", () => 1 - Number.EPSILON);
	assert.equal(randomDigits(), "ffffff");
});

test("randomDigits answers something parseHex accepts", () => {
	for (let i = 0; i < 200; i++) {
		const digits = randomDigits();
		assert.equal(parseHex(digits), `#${digits}`);
	}
});

test("exportFilename takes the name the server sent", () => {
	assert.equal(
		exportFilename('attachment; filename="wavelen-olya-20260912T143005Z.json"'),
		"wavelen-olya-20260912T143005Z.json",
	);
});

test("exportFilename falls back when the header is missing or unquoted", () => {
	assert.equal(exportFilename(null), "wavelen-export.json");
	assert.equal(exportFilename(""), "wavelen-export.json");
	assert.equal(exportFilename("attachment"), "wavelen-export.json");
	assert.equal(exportFilename("attachment; filename=wavelen.json"), "wavelen-export.json");
});
