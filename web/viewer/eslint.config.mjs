import { fixupConfigRules } from "@eslint/compat";
import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";

export default defineConfig([
  ...fixupConfigRules(nextVitals),
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
