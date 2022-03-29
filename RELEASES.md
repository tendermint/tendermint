# Releases

Tendermint uses [semantic versioning](https://semver.org/) with each release following
a `vX.Y.Z` format. The `master` branch is used for active development and thus it's
advisable not to build against it.

The latest changes are always initially merged into `master`.
Releases are specified using tags and are built from long-lived "backport" branches
that are cut from `master` when the release process begins.
Each release "line" (e.g. 0.34 or 0.33) has its own long-lived backport branch,
and the backport branches have names like `v0.34.x` or `v0.33.x`
(literally, `x`; it is not a placeholder in this case). Tendermint only
maintains the last two releases at a time (the oldest release is predominantly
just security patches).

## Backporting

As non-breaking changes land on `master`, they should also be backported
to these backport branches.

We use Mergify's [backport feature](https://mergify.io/features/backports) to automatically backport
to the needed branch. There should be a label for any backport branch that you'll be targeting.
To notify the bot to backport a pull request, mark the pull request with the label corresponding
to the correct backport branch. For example, to backport to v0.35.x, add the label `S:backport-to-v0.35.x`.
Once the original pull request is merged, the bot will try to cherry-pick the pull request
to the backport branch. If the bot fails to backport, it will open a pull request.
The author of the original pull request is responsible for solving the conflicts and
merging the pull request.

### Creating a backport branch

If this is the first release candidate for a major release, you get to have the
honor of creating the backport branch!

Note that, after creating the backport branch, you'll also need to update the
tags on `master` so that `go mod` is able to order the branches correctly. You
should tag `master` with a "dev" tag that is "greater than" the backport
branches tags. See [#6072](https://github.com/tendermint/tendermint/pull/6072)
for more context.

In the following example, we'll assume that we're making a backport branch for
the 0.35.x line.

1. Start on `master`

2. Create and push the backport branch:
   ```sh
   git checkout -b v0.35.x
   git push origin v0.35.x
   ```

3. Create a PR to update the documentation directory for the backport branch.

   We only maintain RFC and ADR documents on master, to avoid confusion.
   In addition, we rewrite Markdown URLs pointing to master to point to the
   backport branch, so that generated documentation will link to the correct
   versions of files elsewhere in the repository. For context on the latter,
   see https://github.com/tendermint/tendermint/issues/7675.

   To prepare the PR:
   ```sh
   # Remove the RFC and ADR documents from the backport.
   # We only maintain these on master to avoid confusion.
   git rm -r docs/rfc docs/architecture

   # Update absolute links to point to the backport.
   go run ./scripts/linkpatch -recur -target v0.35.x -skip-path docs/DOCS_README.md,docs/README.md docs

   # Create and push the PR.
   git checkout -b update-docs-v035x
   git commit -m "Update docs for v0.35.x backport branch." docs
   git push -u origin update-docs-v035x
   ```

   Be sure to merge this PR before making other changes on the newly-created
   backport branch.

After doing these steps, go back to `master` and do the following:

1. Tag `master` as the dev branch for the _next_ major release and push it up to GitHub.
   For example:
   ```sh
   git tag -a v0.36.0-dev -m "Development base for Tendermint v0.36."
   git push origin v0.36.0-dev
   ```

2. Create a new workflow to run e2e nightlies for the new backport branch.
   (See [e2e-nightly-master.yml][e2e] for an example.)

3. Add a new section to the Mergify config (`.github/mergify.yml`) to enable the
   backport bot to work on this branch, and add a corresponding `S:backport-to-v0.35.x`
   [label](https://github.com/tendermint/tendermint/labels) so the bot can be triggered.

4. Add a new section to the Dependabot config (`.github/dependabot.yml`) to
   enable automatic update of Go dependencies on this branch. Copy and edit one
   of the existing branch configurations to set the correct `target-branch`.

[e2e]: https://github.com/tendermint/tendermint/blob/master/.github/workflows/e2e-nightly-master.yml

## Release candidates

Before creating an official release, especially a major release, we may want to create a
release candidate (RC) for our friends and partners to test out. We use git tags to
create RCs, and we build them off of backport branches.

Tags for RCs should follow the "standard" release naming conventions, with `-rcX` at the end
(for example, `v0.35.0-rc0`).

(Note that branches and tags _cannot_ have the same names, so it's important that these branches
have distinct names from the tags/release names.)

If this is the first RC for a major release, you'll have to make a new backport branch (see above).
Otherwise:

1. Start from the backport branch (e.g. `v0.35.x`).
2. Run the integration tests and the e2e nightlies
   (which can be triggered from the Github UI;
   e.g., https://github.com/tendermint/tendermint/actions/workflows/e2e-nightly-34x.yml).
3. Prepare the changelog:
   - Move the changes included in `CHANGELOG_PENDING.md` into `CHANGELOG.md`. Each RC should have
     it's own changelog section. These will be squashed when the final candidate is released.
   - Run `python ./scripts/linkify_changelog.py CHANGELOG.md` to add links for
     all PRs
   - Ensure that `UPGRADING.md` is up-to-date and includes notes on any breaking changes
      or other upgrading flows.
   - Bump TMVersionDefault version in  `version.go`
   - Bump P2P and block protocol versions in  `version.go`, if necessary.
     Check the changelog for breaking changes in these components.
   - Bump ABCI protocol version in `version.go`, if necessary
4. Open a PR with these changes against the backport branch.
5. Once these changes have landed on the backport branch, be sure to pull them back down locally.
6. Once you have the changes locally, create the new tag, specifying a name and a tag "message":
   `git tag -a v0.35.0-rc0 -m "Release Candidate v0.35.0-rc0`
7. Push the tag back up to origin:
   `git push origin v0.35.0-rc0`
   Now the tag should be available on the repo's releases page.
8. Future RCs will continue to be built off of this branch.

Note that this process should only be used for "true" RCs--
release candidates that, if successful, will be the next release.
For more experimental "RCs," create a new, short-lived branch and tag that instead.

## Major release

This major release process assumes that this release was preceded by release candidates.
If there were no release candidates, begin by creating a backport branch, as described above.

1. Start on the backport branch (e.g. `v0.35.x`)
2. Run integration tests (`make test_integrations`) and the e2e nightlies.
3. Prepare the release:
   - "Squash" changes from the changelog entries for the RCs into a single entry,
      and add all changes included in `CHANGELOG_PENDING.md`.
      (Squashing includes both combining all entries, as well as removing or simplifying
      any intra-RC changes. It may also help to alphabetize the entries by package name.)
   - Run `python ./scripts/linkify_changelog.py CHANGELOG.md` to add links for
     all PRs
   - Ensure that `UPGRADING.md` is up-to-date and includes notes on any breaking changes
      or other upgrading flows.
   - Bump TMVersionDefault version in  `version.go`
   - Bump P2P and block protocol versions in  `version.go`, if necessary
   - Bump ABCI protocol version in `version.go`, if necessary
4. Open a PR with these changes against the backport branch.
5. Once these changes are on the backport branch, push a tag with prepared release details.
   This will trigger the actual release `v0.35.0`.
   - `git tag -a v0.35.0 -m 'Release v0.35.0'`
   - `git push origin v0.35.0`
6. Make sure that `master` is updated with the latest `CHANGELOG.md`, `CHANGELOG_PENDING.md`, and `UPGRADING.md`.
7. Add the release to the documentation site generator config (see
   [DOCS_README.md](./docs/DOCS_README.md) for more details). In summary:
   - Start on branch `master`.
   - Add a new line at the bottom of [`docs/versions`](./docs/versions) to
     ensure the newest release is the default for the landing page.
   - Add a new entry to `themeConfig.versions` in
     [`docs/.vuepress/config.js`](./docs/.vuepress/config.js) to include the
	 release in the dropdown versions menu.
   - Commit these changes to `master` and backport them into the backport
     branch for this release.

## Minor release (point releases)

Minor releases are done differently from major releases: They are built off of
long-lived backport branches, rather than from master.  As non-breaking changes
land on `master`, they should also be backported into these backport branches.

Minor releases don't have release candidates by default, although any tricky
changes may merit a release candidate.

To create a minor release:

1. Checkout the long-lived backport branch: `git checkout v0.35.x`
2. Run integration tests (`make test_integrations`) and the nightlies.
3. Check out a new branch and prepare the release:
   - Copy `CHANGELOG_PENDING.md` to top of `CHANGELOG.md`
   - Run `python ./scripts/linkify_changelog.py CHANGELOG.md` to add links for all issues
   - Run `bash ./scripts/authors.sh` to get a list of authors since the latest release, and add the GitHub aliases of external contributors to the top of the CHANGELOG. To lookup an alias from an email, try `bash ./scripts/authors.sh <email>`
   - Reset the `CHANGELOG_PENDING.md`
   - Bump the TMDefaultVersion in `version.go`
   - Bump the ABCI version number, if necessary.
     (Note that ABCI follows semver, and that ABCI versions are the only versions
     which can change during minor releases, and only field additions are valid minor changes.)
4. Open a PR with these changes that will land them back on `v0.35.x`
5. Once this change has landed on the backport branch, make sure to pull it locally, then push a tag.
   - `git tag -a v0.35.1 -m 'Release v0.35.1'`
   - `git push origin v0.35.1`
6. Create a pull request back to master with the CHANGELOG & version changes from the latest release.
   - Remove all `R:minor` labels from the pull requests that were included in the release.
   - Do not merge the backport branch into master.
