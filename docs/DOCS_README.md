# Docs Build Workflow

The documentation for Tendermint Core is hosted at:

- <https://docs.tendermint.com/>

built from the files in this [`docs` directory for `master`](https://github.com/tendermint/tendermint/tree/master/docs)
and other supported release branches.

## How It Works

There is a [GitHub Actions workflow](https://github.com/tendermint/docs/actions/workflows/deployment.yml)
in the `tendermint/docs` repository that clones and builds the documentation
site from the contents of this `docs` directory, for `master` and for the
backport branch of each supported release. Under the hood, this workflow runs
`make build-docs` from the [Makefile](../Makefile#L214).

The list of supported versions are defined in [`config.js`](./.vuepress/config.js),
which defines the UI menu on the documentation site, and also in
[`docs/versions`](./versions), which determines which branches are built.

The last entry in the `docs/versions` file determines which version is linked
by default from the generated `index.html`. This should generally be the most
recent release, rather than `master`, so that new users are not confused by
documentation for unreleased features.

## README

The [README.md](./README.md) is also the landing page for the documentation
on the website. During the Jenkins build, the current commit is added to the bottom
of the README.

## Config.js

The [config.js](./.vuepress/config.js) generates the sidebar and Table of Contents
on the website docs. Note the use of relative links and the omission of
file extensions. Additional features are available to improve the look
of the sidebar.

## Links

**NOTE:** Strongly consider the existing links - both within this directory
and to the website docs - when moving or deleting files.

Links to directories _MUST_ end in a `/`.

Relative links should be used nearly everywhere, having discovered and weighed the following:

### Relative

Where is the other file, relative to the current one?

- works both on GitHub and for the VuePress build
- confusing / annoying to have things like: `../../../../myfile.md`
- requires more updates when files are re-shuffled

### Absolute

Where is the other file, given the root of the repo?

- works on GitHub, doesn't work for the VuePress build
- this is much nicer: `/docs/hereitis/myfile.md`
- if you move that file around, the links inside it are preserved (but not to it, of course)

### Full

The full GitHub URL to a file or directory. Used occasionally when it makes sense
to send users to the GitHub.

## Building Locally

Make sure you are in the `docs` directory and run the following commands:

```bash
rm -rf node_modules
```

This command will remove old version of the visual theme and required packages. This step is optional.

```bash
npm install
```

Install the theme and all dependencies.

```bash
npm run serve
```

<!-- markdown-link-check-disable -->

Run `pre` and `post` hooks and start a hot-reloading web-server. See output of this command for the URL (it is often <https://localhost:8080>).

<!-- markdown-link-check-enable -->

To build documentation as a static website run `npm run build`. You will find the website in `.vuepress/dist` directory.

## Search

We are using [Algolia](https://www.algolia.com) to power full-text search. This uses a public API search-only key in the `config.js` as well as a [tendermint.json](https://github.com/algolia/docsearch-configs/blob/master/configs/tendermint.json) configuration file that we can update with PRs.

## Consistency

Because the build processes are identical (as is the information contained herein), this file should be kept in sync as
much as possible with its [counterpart in the Cosmos SDK repo](https://github.com/cosmos/cosmos-sdk/blob/master/docs/DOCS_README.md).
