# Contributing

Thank you for considering making contributions to Tendermint and related repositories! Start by taking a look at the [coding repo](https://github.com/tendermint/coding) for overall information on repository workflow and standards.

Please follow standard github best practices: fork the repo, branch from the tip of `master`, make some commits, and submit a pull request to `master`.
See the [open issues](https://github.com/tendermint/tendermint/issues) for things we need help with!

Before making a pull request, please open an issue describing the
change you would like to make. If an issue for your change already exists,
please comment on it that you will submit a pull request. Be sure to reference the issue in the opening
comment of your pull request. If your change is substantial, you will be asked
to write a more detailed design document in the form of an
Architectural Decision Record (ie. see [here](./docs/architecture/)) before submitting code
changes.

Please open a [Draft PR](https://github.blog/2019-02-14-introducing-draft-pull-requests/), even if your contribution is incomplete, this inidicates to the community you're working on something and allows them to provide comments early in the development process. When the code is complete it can be marked as ready-for-review.

Please make sure to use `gofmt` before every commit - the easiest way to do this is have your editor run it for you upon saving a file. Additionally please ensure that your code is lint compliant by running `make lint`

## Forking

Please note that Go requires code to live under absolute paths, which complicates forking.
While my fork lives at `https://github.com/ebuchman/tendermint`,
the code should never exist at `$GOPATH/src/github.com/ebuchman/tendermint`.
Instead, we use `git remote` to add the fork as a new remote for the original repo,
`$GOPATH/src/github.com/tendermint/tendermint`, and do all the work there.

For instance, to create a fork and work on a branch of it, I would:

- Create the fork on github, using the fork button.
- Go to the original repo checked out locally (i.e. `$GOPATH/src/github.com/tendermint/tendermint`)
- `git remote rename origin upstream`
- `git remote add origin git@github.com:ebuchman/basecoin.git`

Now `origin` refers to my fork and `upstream` refers to the tendermint version.
So I can `git push -u origin master` to update my fork, and make pull requests to tendermint from there.
Of course, replace `ebuchman` with your git handle.

To pull in updates from the origin repo, run

- `git fetch upstream`
- `git rebase upstream/master` (or whatever branch you want)

## Dependencies

We use [go modules](https://github.com/golang/go/wiki/Modules) to manage dependencies.

That said, the master branch of every Tendermint repository should just build
with `go get`, which means they should be kept up-to-date with their
dependencies so we can get away with telling people they can just `go get` our
software.

Since some dependencies are not under our control, a third party may break our
build, in which case we can fall back on `go mod tidy`. Even for dependencies under our control, go helps us to
keep multiple repos in sync as they evolve. Anything with an executable, such
as apps, tools, and the core, should use dep.

Run `go list -u -m all` to get a list of dependencies that may not be
up-to-date.

When updating dependencies, please only update the particular dependencies you
need. Instead of running `go get -u=patch`, which will update anything,
specify exactly the dependency you want to update, eg.
`GO111MODULE=on go get -u github.com/tendermint/go-amino@master`.

## Vagrant

If you are a [Vagrant](https://www.vagrantup.com/) user, you can get started
hacking Tendermint with the commands below.

NOTE: In case you installed Vagrant in 2017, you might need to run
`vagrant box update` to upgrade to the latest `ubuntu/xenial64`.

```
vagrant up
vagrant ssh
make test
```

## Changelog

Every fix, improvement, feature, or breaking change should be made in a
pull-request that includes an update to the `CHANGELOG_PENDING.md` file.

Changelog entries should be formatted as follows:

```
- [module] \#xxx Some description about the change (@contributor)
```

Here, `module` is the part of the code that changed (typically a
top-level Go package), `xxx` is the pull-request number, and `contributor`
is the author/s of the change.

It's also acceptable for `xxx` to refer to the relevent issue number, but pull-request
numbers are preferred.
Note this means pull-requests should be opened first so the changelog can then
be updated with the pull-request's number.
There is no need to include the full link, as this will be added
automatically during release. But please include the backslash and pound, eg. `\#2313`.

Changelog entries should be ordered alphabetically according to the
`module`, and numerically according to the pull-request number.

Changes with multiple classifications should be doubly included (eg. a bug fix
that is also a breaking change should be recorded under both).

Breaking changes are further subdivided according to the APIs/users they impact.
Any change that effects multiple APIs/users should be recorded multiply - for
instance, a change to the `Blockchain Protocol` that removes a field from the
header should also be recorded under `CLI/RPC/Config` since the field will be
removed from the header in rpc responses as well.

## Branching Model and Release

The main development branch is master.

Every release is maintained in a release branch named `vX.Y.Z`.

Note all pull requests should be squash merged except for merging to a release branch (named `vX.Y`). This keeps the commit history clean and makes it
easy to reference the pull request where a change was introduced.

### Development Procedure

- the latest state of development is on `master`
- `master` must never fail `make test`
- never --force onto `master` (except when reverting a broken commit, which should seldom happen)
- create a development branch either on github.com/tendermint/tendermint, or your fork (using `git remote add origin`)
- make changes and update the `CHANGELOG_PENDING.md` to record your change
- before submitting a pull request, run `git rebase` on top of the latest `master`

### Pull Merge Procedure

- ensure pull branch is based on a recent `master`
- run `make test` to ensure that all tests pass
- squash merge pull request
- the `unstable` branch may be used to aggregate pull merges before fixing tests

### Release Procedure

#### Major Release

1. start on `master`
2. run integration tests (see `test_integrations` in Makefile)
3. prepare release in a pull request against `master` (to be squash merged):
   - copy `CHANGELOG_PENDING.md` to top of `CHANGELOG.md`
   - run `python ./scripts/linkify_changelog.py CHANGELOG.md` to add links for
     all issues
   - run `bash ./scripts/authors.sh` to get a list of authors since the latest
     release, and add the github aliases of external contributors to the top of
     the changelog. To lookup an alias from an email, try `bash ./scripts/authors.sh <email>`
   - reset the `CHANGELOG_PENDING.md`
   - bump versions
4. push your changes with prepared release details to `vX.X` (this will trigger the release `vX.X.0`)
5. merge back to master (don't squash merge!)

#### Minor Release

If there were no breaking changes and you need to create a release nonetheless,
the procedure is almost exactly like with a new release above.

The only difference is that in the end you create a pull request against the existing `X.X` branch.
The branch name should match the release number you want to create.
Merging this PR will trigger the next release.
For example, if the PR is against an existing 0.34 branch which already contains a v0.34.0 release/tag,
the patch version will be incremented and the created release will be v0.34.1.

#### Backport Release

1. start from the existing release branch you want to backport changes to (e.g. v0.30)
   Branch to a release/vX.X.X branch locally (e.g. release/v0.30.7)
2. cherry pick the commit(s) that contain the changes you want to backport (usually these commits are from squash-merged PRs which were already reviewed)
3. steps 2 and 3 from [Major Release](#major-release)
4. push changes to release/vX.X.X branch
5. open a PR against the existing vX.X branch

## Testing

All repos should be hooked up to [CircleCI](https://circleci.com/).

If they have `.go` files in the root directory, they will be automatically
tested by circle using `go test -v -race ./...`. If not, they will need a
`circle.yml`. Ideally, every repo has a `Makefile` that defines `make test` and
includes its continuous integration status using a badge in the `README.md`.

### RPC Testing

If you contribute to the RPC endpoints it's important to document your changes in the [Swagger file](./docs/spec/rpc/swagger.yaml)
To test your changes you should install `nodejs` and run:

```bash
npm i -g dredd
make build-linux build-contract-tests-hooks
make contract-tests
```

This command will popup a network and check every endpoint against what has been documented
