import { defineConfig } from 'i18next-cli';

export default defineConfig({
  "locales": [
    "en",
    "zh",
    "zh-tw",
    "fr"
  ],
  "extract": {
    "input": [
      "api/**/*.{js,jsx,ts,tsx}",
      "config/**/*.{js,jsx,ts,tsx}",
      "components/**/*.{js,jsx,ts,tsx}",
      "pages/**/*.{js,jsx,ts,tsx}"
    ],
    "output": "i18n/locales/{{language}}.json",
    "defaultNS": "translation",
    "functions": [
      "t",
      "*.t"
    ],
    "transComponents": [
      "Trans"
    ]
  },
  "types": {
    "input": [
      "locales/{{language}}/{{namespace}}.json"
    ],
    "output": "src/types/i18next.d.ts"
  }
});