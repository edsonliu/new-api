# JDOTOKEN Git tag delivery

Production: `115.29.196.54`, application `jdtoken-new-api-prod`.
Release tags: annotated, immutable `jdtoken-vYYYY.MM.DD.N` (production only).
These tags are excluded from the upstream Docker Hub / desktop release flows.

1. Merge upstream in an isolated worktree; run backend, frontend, database,
   pricing and security regressions. Build the custom frontend and backend with
   the Go version required by upstream. Retain upstream licenses and attribution.
2. Build an image whose revision label is the final source commit and whose
   version label is the intended release tag. Transfer the image to production
   without switching the application.
3. Run `stage.py prepare TAG`, `stage.py start TAG`, and `stage.py verify TAG`
   after restarting the rehearsal app. This uses an isolated network and a copy
   of production PostgreSQL. All copied channels are disabled.
4. Store `manifest.json` and `stage-result.json` in the root-owned directory
   `/data/app/jdtoken-new-api/git-release/artifacts/TAG/`. The manifest includes
   `commit`, `version`, `image`, `image_id`, `target`, and `validated: true`.
   Mark validated only after tests and rehearsal pass; keep logs with the release.
5. Install the reviewed `receive.py` and `deploy.py` under `git-release/bin/`.
   The bare repository at `git-release/repository.git` has pre-receive and
   post-receive hooks calling `receive.py pre` and `receive.py post` respectively.
   A dedicated SSH key is restricted to git-receive-pack for that repository.
6. Push the source branch and annotated release tag to the GitHub origin, then
   push that same tag to the `production` Git remote. This second Git tag push
   triggers the deployment pipeline; branch pushes cannot deploy. Both remotes
   must identify the same tag object and source commit.
7. The receiver rejects tag replacement/deletion, unvalidated artifacts,
   incorrect commits/targets, and delivery scripts different from tagged source.
   The systemd job verifies image identity, drains requests, backs up PostgreSQL
   and configuration, deploys, and checks both public domains. On failure it
   restores the prior image/configuration without restoring over new transactions.

Inspect `journalctl -u jdtoken-release-TAG` and the artifact directory's
`deployment-result.json`. A successful Git push means the job was queued, not
that deployment succeeded. Never deploy a failed tag by manually switching images.
Prepare a corrected, newly validated release tag instead. Automatic rollback is
part of the tag-triggered job, not an alternative release path.

The 2026-10-22 upgrade adds an audit index and two cache-usage columns to the
quota statistics table. The prior image can ignore these additions. Reassess migration and rollback
compatibility on every future upgrade. Remove temporary stage containers after
acceptance, but retain production backups, manifests and test evidence.

This extension changes no application authentication, billing or channel logic.
Future upstream merges must retain the two workflow tag exclusions and review
the dedicated delivery scripts against current Compose/Nginx configuration.

Seedance phase 1 initial activation may add a root-only `runtime.env` artifact.
Its SHA256 must match the manifest and production-copy rehearsal. The tag job
adds the four `JDOTOKEN_SEEDANCE_*` variables via `seedance.env`, uses the
existing persistent `/data` volume, and changes Nginx access logging to omit
query parameters. The job backs up and restores these files on failure. This
initial activation deliberately rejects overriding pre-existing runtime values;
subsequent capability changes use the native `JDOTOKENSeedance` option. Never
commit or copy the signing secret into public release evidence.

## Static documentation releases (2026-10-23)

Documentation source now lives in `jdtoken/gateway-docs/`. A `release_type: "docs"`
manifest uses the **same annotated production tag and receiver**, but publishes only
the independently served static documentation. Application releases still require
all image build, production database copy, migration and runtime checks above.

For docs-only releases, the diff from `app_commit` is restricted to
`jdtoken/gateway-docs/` and `deployment/jdtoken/`. This prevents an application change
from being silently skipped by documentation delivery. Record the live application's
exact source revision and image ID, plus its container identity and version. This
branch does not build an application image, migrate/copy the database, restart an
application, alter Compose/Nginx/runtime settings, or interrupt model requests.
Its rehearsal uses a copy of the **production static site**, followed by a candidate
switch and rollback in an isolated staging directory.

1. Build docs, run syntax/browser checks and receiver/publish/rollback regressions.
   Set the docs `release.json` version in `build.py` to the tag without `jdtoken-v`.
2. Commit sources, then place built files (regular files only) in
   `git-release/artifacts/TAG/docs/`. The manifest contains `release_type: "docs"`,
   `commit`, `version`, `target`, `validated: true`, `app_commit`, `app_image_id`, and
   `docs_files` (relative filename → SHA256). No image replacement field is used.
3. Rehearse the copied static site: verify HTTP responses against `docs_files`,
   switch and restore the staging symlink, check readable file modes and unchanged
   production application identity. Store `stage-result.json` with `success: true`,
   `version`, `release_type: "docs"`, `app_image_id`, `docs_files` and test evidence.
4. Back up and install matching `receive.py`, `deploy.py` and `docs_release.py` into
   `git-release/bin/`; the tag receiver verifies all three against committed source.
5. Push origin branch and annotated tag, then the same tag to production. The
   receiver rebuilds docs from the exact tag in an isolated temporary directory and
   compares every output byte against the staged artifact. It rejects changed files,
   forged artifact hashes, symlinks, wrong app baseline and reused release tags.
6. The job rechecks the running application's commit/image, backs up the complete
   old static release and pointer record, and publishes under
   `/data/www/jdtoken-docs/releases/TAG/docs/`. It atomically replaces `current`,
   verifies every public file on both domains, and checks unchanged container ID,
   start time, image, version and restart count. Failure restores the prior pointer.
   Inspect systemd and `deployment-result.json` exactly as for application releases.

The docs tag identifies the **documentation** release. `/api/status` continues to
report the existing application image version; do not misrepresent a docs update
as an application rebuild. Later application releases remain compatible with the
receiver's default `release_type: "app"` path. Successful docs rollback uses a new
validated tag with reverted content; automatic failure rollback remains part of
the tag job itself. Backups include no database copy because this branch does not
read or mutate database contents.
