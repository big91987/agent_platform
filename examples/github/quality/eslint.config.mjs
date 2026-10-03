export default [
  {
    files: ["**/*.js"],
    languageOptions: {
      ecmaVersion: "latest",
      sourceType: "script",
      globals: Object.fromEntries(
        [
          "document",
          "window",
          "localStorage",
          "crypto",
          "console",
          "alert",
          "confirm",
          "setTimeout",
          "clearTimeout",
        ].map((name) => [name, "readonly"]),
      ),
    },
    rules: {
      "no-undef": "error",
      "no-unused-vars": ["error", { argsIgnorePattern: "^_" }],
      "no-unreachable": "error",
      "valid-typeof": "error",
      eqeqeq: "error",
    },
  },
];
