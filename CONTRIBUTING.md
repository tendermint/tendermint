# Contributing

Thank you for considering making contributions to Tendermint and related repositories! Start by taking a look at the [coding repo](https://github.com/tendermint/coding) for overall information on repository workflow and standards.

Please follow standard github best practices: fork the repo, branch from the tip of develop, make some commits, and submit a pull request to develop. See the [open issues](https://github.com/tendermint/tendermint/issues) for things we need help with!

Please make sure to use `gofmt` before every commit - the easiest way to do this is have your editor run it for you upon saving a file.

## Forking

Please note that Go requires code to live under absolute paths, which complicates forking.
While my fork lives at `https://github.com/ebuchman/tendermint`,
the code should never exist at  `$GOPATH/src/github.com/ebuchman/tendermint`.
Instead, we use `git remote` to add the fork as a new remote for the original repo,
`$GOPATH/src/github.com/tendermint/tendermint `, and do all the work there.

For instance, to create a fork and work on a branch of it, I would:

  * Create the fork on github, using the fork button.
  * Go to the original repo checked out locally (i.e. `$GOPATH/src/github.com/tendermint/tendermint`)
  * `git remote rename origin upstream`
  * `git remote add origin git@github.com:ebuchman/basecoin.git`

Now `origin` refers to my fork and `upstream` refers to the tendermint version.
So I can `git push -u origin master` to update my fork, and make pull requests to tendermint from there.
Of course, replace `ebuchman` with your git handle.

To pull in updates from the origin repo, run

  * `git fetch upstream`
  * `git rebase upstream/master` (or whatever branch you want)

Please don't make Pull Requests to `master`.

## Dependencies

We use [dep](https://github.com/golang/dep) to manage dependencies.

That said, the master branch of every Tendermint repository should just build
with `go get`, which means they should be kept up-to-date with their
dependencies so we can get away with telling people they can just `go get` our
software.

Since some dependencies are not under our control, a third party may break our
build, in which case we can fall back on `dep ensure` (or `make
get_vendor_deps`). Even for dependencies under our control, dep helps us to
keep multiple repos in sync as they evolve. Anything with an executable, such
as apps, tools, and the core, should use dep.

Run `dep status` to get a list of vendor dependencies that may not be
up-to-date.

When updating dependencies, please only update the particular dependencies you
need. Instead of running `dep ensure -update`, which will update anything,
specify exactly the dependency you want to update, eg.
`dep ensure -update github.com/tendermint/go-amino`.

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

All repos should adhere to the branching model: http://nvie.com/posts/a-successful-git-branching-model/.
This means that all pull-requests should be made against develop. Any merge to
master constitutes a tagged release.

### Development Procedure:
- the latest state of development is on `develop`
- `develop` must never fail `make test`
- never --force onto `develop` (except when reverting a broken commit, which should seldom happen)
- create a development branch either on github.com/tendermint/tendermint, or your fork (using `git remote add origin`)
- make changes and update the `CHANGELOG_PENDING.md` to record your change
- before submitting a pull request, run `git rebase` on top of the latest `develop`

### Pull Merge Procedure:
- ensure pull branch is based on a recent develop
- run `make test` to ensure that all tests pass
- merge pull request
- the `unstable` branch may be used to aggregate pull merges before fixing tests

### Release Procedure:
- start on `develop`
- run integration tests (see `test_integrations` in Makefile)
- prepare changelog:
    - copy `CHANGELOG_PENDING.md` to top of `CHANGELOG.md`
    - run `python ./scripts/linkify_changelog.py CHANGELOG.md` to add links for
      all issues
    - run `bash ./scripts/authors.sh` to get a list of authors since the latest
      release, and add the github aliases of external contributors to the top of
      the changelog. To lookup an alias from an email, try `bash
      ./scripts/authors.sh <email>`
    - reset the `CHANGELOG_PENDING.md`
- bump versions
- push to release/vX.X.X to run the extended integration tests on the CI
- merge to master
- merge master back to develop

### Hotfix Procedure:
- start on `master`
- checkout a new branch named hotfix-vX.X.X
- make the required changes
  - these changes should be small and an absolute necessity
  - add a note to CHANGELOG.md
- bump versions
- push to hotfix-vX.X.X to run the extended integration tests on the CI
- merge hotfix-vX.X.X to master
- merge hotfix-vX.X.X to develop
- delete the hotfix-vX.X.X branch


## Testing

All repos should be hooked up to [CircleCI](https://circleci.com/).

If they have `.go` files in the root directory, they will be automatically
tested by circle using `go test -v -race ./...`. If not, they will need a
`circle.yml`. Ideally, every repo has a `Makefile` that defines `make test` and
includes its continuous integration status using a badge in the `README.md`.
