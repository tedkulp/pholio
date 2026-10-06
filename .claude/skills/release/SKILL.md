---
name: release
description: Use when cutting a release of pholio: running the CI gate, moving the changelog's Unreleased entries into a dated section, tagging, and pushing so GoReleaser publishes. Covers what the tag triggers and how to verify it landed.
---

# Releasing pholio

A release is a pushed `v*` tag. `.github/workflows/release.yml` runs GoReleaser,
which builds linux and darwin binaries for amd64 and arm64, publishes a GitHub
Release with archives, checksums, `.deb` and `.rpm`, and updates the Homebrew
cask in `tedkulp/homebrew-tap`. Everything after the push is automatic; this
skill is the part that is not.

Work through the steps in order. **A pushed tag cannot be taken back**: a
re-tag republishes over a release people may already have fetched.

## Step 1: Check the tree

```sh
git status --short
git fetch && git status -sb | head -1
```

Release from `main`, clean and up to date with `origin/main`. Stop and ask if
there are uncommitted changes or the branch is ahead/behind.

## Step 2: Run the gate

```sh
just ci
just release-check
```

`just ci` is vet, race tests and golangci-lint, the same as CI. `release-check`
validates `.goreleaser.yaml`. Both must pass; do not tag on red.

## Step 3: Decide the version

```sh
git tag --sort=-version:refname | head -5
git log --oneline $(git describe --tags --abbrev=0 2>/dev/null)..HEAD
```

Semver against the last tag (no tags yet means the first release is `v0.1.0`):

- **Patch** (`0.1.0 → 0.1.1`): fixes only.
- **Minor** (`0.1.x → 0.2.0`): new features, backwards-compatible.
- **Major** (`0.x.x → 1.0.0`): a break in Vault format, config, or keymap users rely on.

If the user passed a version as the argument, use it. Otherwise propose one with
the reasoning and confirm before tagging.

## Step 4: Move the changelog's Unreleased entries

`CHANGELOG.md` follows Keep a Changelog. Add a dated section directly below
`## [Unreleased]`, and leave `## [Unreleased]` in place and empty:

```markdown
## [Unreleased]

## [0.2.0] - 2026-10-06

### Added
- ...
```

Heading format: `## [0.2.0] - YYYY-MM-DD`, bracketed, no leading `v`, hyphen.
Categories: `Added`, `Changed`, `Deprecated`, `Removed`, `Fixed`, `Security`.

If Unreleased is missing things, fill it from the commit log in Step 3, written
for users in `CONTEXT.md` vocabulary (Note, Vault, Zettel, Task List…), not as
commit subjects.

Use today's real date:

```sh
date -u +%Y-%m-%d
```

## Step 5: Commit

```sh
git add CHANGELOG.md
git commit -m "Release v0.2.0: <one-line summary of what changed>"
```

Commit before tagging, so the tag contains the changelog it describes.

## Step 6: Tag and push both

```sh
git tag -a v0.2.0 -m "Release v0.2.0"
git push && git push origin v0.2.0
```

`git push` alone does **not** carry the tag. The second command starts the release.

## Step 7: Verify

```sh
gh run list --workflow release.yml --limit 3
gh run watch <run-id>
gh release view v0.2.0
```

Expect eight archives and packages plus `checksums.txt`: `tar.gz` for
linux/darwin × amd64/arm64, and `.deb` and `.rpm` for the two linux arches.
Then confirm the cask updated:

```sh
gh api repos/tedkulp/homebrew-tap/contents/Casks/pholio.rb --jq .download_url
```

And that the binary knows its version:

```sh
gh release download v0.2.0 -p '*_linux_amd64.tar.gz' -O - | tar -xzO pholio > /tmp/pholio && chmod +x /tmp/pholio && /tmp/pholio --version
# pholio v0.2.0
```

## The one secret this repository needs

`HOMEBREW_TOKEN`, with push access to `tedkulp/homebrew-tap`. Everything else
uses the run's own `GITHUB_TOKEN`.

```sh
gh secret list --repo tedkulp/pholio
```

If it is missing, the job fails at its **last** step, after the GitHub Release
has already published: most of the release worked, only the cask is stale. Set
the secret and re-run rather than re-tagging:

```sh
gh secret set HOMEBREW_TOKEN --repo tedkulp/pholio
gh run rerun <run-id> --failed
```

## When it goes wrong

- **`pholio --version` prints `(devel)` or a pseudo-version.** The `-X main.version`
  stamp in `.goreleaser.yaml` no longer matches the `version` var in
  `cmd/pholio/main.go`. `go build` does not error on a missing `-X` symbol.
- **The tag was pushed with a broken config.** If the release did not publish,
  delete the local and remote tag (`git tag -d vX.Y.Z && git push origin :vX.Y.Z`),
  fix, and re-tag. If it did publish, release a new patch version instead.

## Checking the config without releasing

```sh
just release-check   # goreleaser check
just snapshot        # full local build into dist/, publishing nothing
```

## Quick reference

| Step | Command |
|---|---|
| Gate | `just ci && just release-check` |
| Last tags | `git tag --sort=-version:refname \| head -5` |
| Today, in UTC | `date -u +%Y-%m-%d` |
| Commit | `git commit -m "Release vX.Y.Z: <summary>"` |
| Tag | `git tag -a vX.Y.Z -m "Release vX.Y.Z"` |
| Push both | `git push && git push origin vX.Y.Z` |
| Watch | `gh run list --workflow release.yml --limit 3` |
| Inspect | `gh release view vX.Y.Z` |
