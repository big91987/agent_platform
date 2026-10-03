# Project Agent instructions (merge into the target project's AGENTS.md)

Describe the product, its actual commands, data contracts and acceptance plans in
this project's docs/README.md. Keep scope limited to the requested product change.

When the prompt explicitly identifies a Harness CI/CD managed task workspace, the
Runner owns branches, commits, pushes and PR creation. Work in the supplied cwd;
do not create/switch branches or publish directly. Follow the current stage's
instructions and registered handoff tools. Outside that managed environment,
follow this repository's normal local Git policy.

Preserve existing data and behavior unless the accepted task changes them. Run
actual required checks and record failures and untested limitations honestly.
Do not change .github/, shared execution tooling or validation controls to make a
product check pass. Maintain product tests/browser/core.json locators when UI
changes, retaining its intended business actions and assertions.
