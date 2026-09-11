import js from "@eslint/js";
import globals from "globals";
import { defineConfig } from "eslint/config";

// app.js and lib.js are ES modules, loaded by the one <script type="module"> in index.html. The
// inline script in index.html is not linted, eslint reads no HTML.
export default defineConfig([
	{
		files: ["app.js", "lib.js"],
		extends: [js.configs.recommended],
		languageOptions: {
			ecmaVersion: "latest",
			sourceType: "module",
			globals: globals.browser,
		},
	},
	// tests run under node --test, they don't see the browser's globals or DOM
	{
		files: ["*.test.js"],
		extends: [js.configs.recommended],
		languageOptions: {
			ecmaVersion: "latest",
			sourceType: "module",
			globals: globals.nodeBuiltin,
		},
	},
]);
