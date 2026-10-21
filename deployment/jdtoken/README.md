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
