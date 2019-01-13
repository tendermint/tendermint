# Docs Build Workflow

The documentation for Tendermint Core is hosted at:

- https://tendermint.com/docs/ and
- https://tendermint-staging.interblock.io/docs/

built from the files in this (`/docs`) directory for
[master](https://github.com/tendermint/tendermint/tree/master/docs)
and [develop](https://github.com/tendermint/tendermint/tree/develop/docs),
respectively.

## How It Works

There is a CircleCI job listening for changes in the `/docs` directory, on both
the  `master` and `develop` branches. Any updates to files in this directory
on those branches will automatically trigger a website deployment. Under the hood,
the private website repository has a `make build-docs` target consumed by a CircleCI job in that repo.

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

Links to directories *MUST* end in a `/`.

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

To build and serve the documentation locally, run:

```
# from this directory
npm install
npm install -g vuepress
```

then change the following line in the `config.js`:

```
base: "/docs/",
```

to:

```
base: "/",
```

Finally, go up one directory to the root of the repo and run:

```
# from root of repo
vuepress build docs
cd dist/docs
python -m SimpleHTTPServer 8080
```

then navigate to localhost:8080 in your browser.

## Search

We are using [Algolia](https://www.algolia.com) to power full-text search. This uses a public API search-only key in the `config.js` as well as a [tendermint.json](https://github.com/algolia/docsearch-configs/blob/master/configs/tendermint.json) configuration file that we can update with PRs.

## Consistency

Because the build processes are identical (as is the information contained herein), this file should be kept in sync as
much as possible with its [counterpart in the Cosmos SDK repo](https://github.com/cosmos/cosmos-sdk/blob/develop/docs/DOCS_README.md).
