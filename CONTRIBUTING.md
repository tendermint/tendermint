# Contributing

Thank you for your interest in contributing to Tendermint! Before
contributing, it may be helpful to understand the goal of the project. The goal
of Tendermint is to develop a BFT consensus engine robust enough to
support permissionless value-carrying networks. While all contributions are
welcome, contributors should bear this goal in mind in deciding if they should
target the main tendermint project or a potential fork. When targeting the
main Tendermint project, the following process leads to the best chance of
landing changes in master.

All work on the code base should be motivated by a [Github
Issue](https://github.com/tendermint/tendermint/issues).
[Search](https://github.com/tendermint/tendermint/issues?q=is%3Aopen+is%3Aissue+label%3A%22help+wanted%22)
is a good place start when looking for places to contribute. If you
would like to work on an issue which already exists, please indicate so
by leaving a comment.

All new contributions should start with a [Github
Issue](https://github.com/tendermint/tendermint/issues/new/choose). The
issue helps capture the problem you're trying to solve and allows for
early feedback. Once the issue is created the process can proceed in different
directions depending on how well defined the problem and potential
solution are. If the change is simple and well understood, maintainers
will indicate their support with a heartfelt emoji.

If the issue would benefit from thorough discussion, maintainers may
request that you create a [Request For
Comment](https://github.com/tendermint/spec/tree/master/rfc). Discussion
at the RFC stage will build collective understanding of the dimensions
of the problems and help structure conversations around trade-offs.

When the problem is well understood but the solution leads to large
structural changes to the code base, these changes should be proposed in
the form of an [Architectural Decision Record
(ADR)](./docs/architecture/). The ADR will help build consensus on an
overall strategy to ensure the code base maintains coherence
in the larger context. If you are not comfortable with writing an ADR,
you can open a less-formal issue and the maintainers will help you
turn it into an ADR. ADR numbers can be registered [here](https://github.com/tendermint/tendermint/issues/2313).

When the problem as well as proposed solution are well understood,
changes should start with a [draft
pull request](https://github.blog/2019-02-14-introducing-draft-pull-requests/)
against master. The draft signals that work is underway. When the work
is ready for feedback, hitting "Ready for Review" will signal to the
maintainers to take a look.

![Contributing flow](./docs/imgs/contributing.png)

Each stage of the process is aimed at creating feedback cycles which align contributors and maintainers to make sure:

- Contributors don’t waste their time implementing/proposing features which won’t land in master.
- Maintainers have the necessary context in order to support and review contributions.

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

## Protobuf

We use [Protocol Buffers](https://developers.google.com/protocol-buffers) along with [gogoproto](https://github.com/gogo/protobuf) to generate code for use across Tendermint Core.

For linting and checking breaking changes, we use [buf](https://buf.build/). If you would like to run linting and check if the changes you have made are breaking then you will need to have docker running locally. Then the linting cmd will be `make proto-lint` and the breaking changes check will be `make proto-check-breaking`.

There are two ways to generate your proto stubs.

1. Use Docker, pull an image that will generate your proto stubs with no need to install anything. `make proto-gen-docker`
2. Run `make proto-gen` after installing `protoc` and gogoproto.

### Installation Instructions

To install `protoc`, download an appropriate release (https://github.com/protocolbuffers/protobuf) and then move the provided binaries into your PATH (follow instructions in README included with the download).

To install `gogoproto`, do the following:

```sh
$ go get github.com/gogo/protobuf/gogoproto
$ cd $GOPATH/pkg/mod/github.com/gogo/protobuf@v1.3.1 # or wherever go get installs things
$ make install
```

You should now be able to run `make proto-gen` from inside the root Tendermint directory to generate new files from proto files.

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

When you have submitted a pull request label the pull request with either `R:minor`, if the change can be accepted in a minor release, or `R:major`, if the change is meant for a major release.

### Pull Merge Procedure

- ensure pull branch is based on a recent `master`
- run `make test` to ensure that all tests pass
- [squash](https://stackoverflow.com/questions/5189560/squash-my-last-x-commits-together-using-git) merge pull request
- the `unstable` branch may be used to aggregate pull merges before fixing tests

### Git Commit Style

We follow the [Go style guide on commit messages](https://tip.golang.org/doc/contribute.html#commit_messages). Write concise commits that start with the package name and have a description that finishes the sentence "This change modifies Tendermint to...". For example,

\```
cmd/debug: execute p.Signal only when p is not nil

[potentially longer description in the body]

Fixes #nnnn
\```

Each PR should have one commit once it lands on `master`; this can be accomplished by using the "squash and merge" button on Github. Be sure to edit your commit message, though!

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
   - bump the appropriate versions in `version.go`
4. push your changes with prepared release details to `vX.X` (this will trigger the release `vX.X.0`)
5. merge back to master (don't squash merge!)

#### Minor Release

Minor releases are done differently from major releases. Minor release pull requests should be labeled with `R:minor` if they are to be included.

1. Checkout the last major release, `vX.X`.

   - `git checkout vX.X`

2. Create a release candidate branch off the most recent major release with your upcoming version specified, `rc1/vX.X.x`, and push the branch.

   - `git checkout -b rc1/vX.X.x`
   - `git push -u origin rc1/vX.X.x`

3. Create a cherry-picking branch, and make a pull request into the release candidate.

   - `git checkout -b cherry-picks/rc1/vX.X.x`

     - This is for devs to approve the commits that are entering the release candidate.
     - There may be merge conflicts.

4. Begin cherry-picking.

   - `git cherry-pick {PR commit from master you wish to cherry pick}`
   - Fix conflicts
   - `git cherry-pick --continue`
   - `git push cherry-picks/rc1/vX.X.x`

   > Once all commits are included and CI/tests have passed, then it is ready for a release.

5. Create a release branch `release/vX.X.x` off the release candidate branch.

   - `git checkout -b release/vX.X.x`
   - `git push -u origin release/vX.X.x`
     > Note this Branch is protected once pushed, you will need admin help to make any change merges into the branch.

6. Merge Commit the release branch into the latest major release branch `vX.X`, this will start the release process.

7. Create a Pull Request back to master with the CHANGELOG & version changes from the latest release.
   - Remove all `R:minor` labels from the pull requests that were included in the release.
     > Note: Do not merge the release branch into master.

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

If you contribute to the RPC endpoints it's important to document your changes in the [Swagger file](./rpc/swagger/swagger.yaml)
To test your changes you should install `nodejs` and run:

```bash
npm i -g dredd
make build-linux build-contract-tests-hooks
make contract-tests
```

This command will popup a network and check every endpoint against what has been documented
