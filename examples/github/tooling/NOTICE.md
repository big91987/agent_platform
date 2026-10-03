# Source snapshot

This directory bundles the minimum import closure used by the GitHub example:
`browser.py`, `common.py`, `console.py`, `quality.py`, their configuration and
locked JavaScript browser runtime. The files were copied byte-for-byte from the
owner-maintained `big91987/he_skeleton` checkout. `SOURCE.json` records the exact
source commit and SHA-256 for each included file; it includes the previously
validated browser-interaction fixes. No other Harness workflow/router is bundled.

The source checkout did not contain a standalone LICENSE file at packaging time.
This notice records provenance and does not assign or invent a new license.
Playwright and its transitive dependencies retain their own upstream licenses;
install them from the lock file rather than committing node_modules.

Keep this snapshot immutable when possible. To update, select and verify an
upstream revision, copy the same dependency closure, refresh SOURCE.json, run the
portable import/browser checks and the adapter suite, then review the diff.
Do not copy a local runtime directory, credentials or tool caches.
