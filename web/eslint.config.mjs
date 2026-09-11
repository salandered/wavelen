import js from "@eslint/js";
import globals from "globals";
import { defineConfig } from "eslint/config";

// app.js is a classic script, not a module: every top-level name is a global, which is what lets
// no-undef catch a typo. The inline script in index.html is not linted, eslint reads no HTML.
export default defineConfig([
	{
		files: ["app.js"],
		extends: [js.configs.recommended],
		languageOptions: {
			ecmaVersion: "latest",
			sourceType: "script",
			globals: globals.browser,
		},
	},
]);
