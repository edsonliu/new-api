# JDOTOKEN public gateway documentation

Authoritative source for https://www.jdotoken.com/docs/ and https://jdotoken.com/docs/.
The previous workspace-level `gateway-docs/` is a historical copy. Edit this versioned
module so documentation can be traced to an immutable production Git tag.

`content.py` owns customer-facing descriptions and examples. `build.py` renders the
static site; `assets/` contains independent styles, interactions and brand images.
No application runtime, production credentials, SDK or dependency bundle is loaded.
Existing company logo, ICP and public-security registration links are retained.

## Build and check

```sh
python3 jdtoken/gateway-docs/build.py
# Serve the generated dist directory at /docs/ on a local HTTP server.
python3 jdtoken/gateway-docs/tests/check.py --url http://127.0.0.1:4182/docs/
python3 -m unittest discover -s deployment/jdtoken -p test_receive.py
```

Build output `dist/` and syntax-check inventory `examples.json` are untracked.
The browser check uses Python Playwright, Chrome, Bash and Node; it validates
example syntax without executing generation requests. It checks layout at desktop,
390px and 320px, all anchors, search, copying, keyboard tabs and resource loading.

## Content baseline

Updated 2026-10-26 against `jdtoken/seedance/` and production acceptance evidence in
workspace `reports/seedance-release-20261023/`,
`reports/seedance-media-e2e-20261024/`, `reports/lingxi-audio-reference-20261024/`
`reports/lingxi-full-audio-20261026/` and `reports/seedance-audio-matrix-20261026/`. Documents ten material Actions,
capabilities, signed uploads, references, pagination, cancellation and local hiding.
Service identifiers are public aliases, not internal channel IDs or credentials.
Keep configured capabilities separate from actual end-to-end tests, particularly
2.5 draft upgrades, unverified audio models, automatic ingestion, real-person verification and callbacks.
Standard audio URL storage and references for all four models, plus alternate 2.0 audio storage/references, have passed acceptance;
local audio uploads remain disabled. Document these as separate capabilities.
Always use platform IDs. Pricing links point to the model plaza rather than a frozen
price table. API paths, versions, ownership, restrictions and response examples must
be reviewed against implementation and fresh evidence after functional changes.

## Publish and roll back

Follow `deployment/jdtoken/README.md` under its static docs release procedure.
Use an annotated `jdtoken-vYYYY.MM.DD.N` tag, first origin then production. Do not
replay historical workspace `deploy-remote.py` or `publish-update-remote.py` scripts.
The docs job backs up the prior static release and atomically changes its symlink;
it validates both public domains and automatically restores the pointer on failure.
A successful publication can be reverted by publishing a new validated tag with
reverted content. Keep the old release and backup until the retention policy allows
removal; never overwrite an existing tag or manually switch images to roll back docs.

This module adds no upstream application integration points, database changes,
pricing logic or dependencies. Future upgrades must preserve the dedicated Git tag
receiver/docs branch and Nginx `/docs/` static location. Review API examples, route
semantics, navigation/anchors, CSP and desktop/mobile checks after upstream merges.
