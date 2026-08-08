import { fixupConfigRules } from "@eslint/compat";
import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import * as espree from "espree";

export default defineConfig([
  ...fixupConfigRules(nextVitals),
  {
    files: ["**/*.{js,mjs,cjs,jsx}"],
    languageOptions: {
      // eslint-config-next/parser 16.3.0 does not yet implement ESLint 10's
      // ScopeManager#addGlobals contract. Espree is ESLint's native JS parser
      // and preserves the Next/React/a11y rule configuration above.
      parser: espree,
    },
  },
  {
    rules: {
      // React Compiler is not enabled; existing effect-driven loaders remain covered by unit and browser tests.
      "react-hooks/set-state-in-effect": "off",
    },
  },
  {
    files: ["test/**/*.tsx", "__tests__/**/*.tsx"],
    rules: {
      "@next/next/no-img-element": "off",
    },
  },
  globalIgnores([".next/**", "playwright-report/**", "test-results/**"]),
]);
