# RFC 019: Configuration File Versioning

## Changelog

- 19-Apr-2022: Initial draft (@creachadair)

## Abstract

Updating configuration settings is an essential part of upgrading an existing
node to a new version of the Tendermint software.  Unfortunately, it is also
currently a very manual process. This document discusses some of the history of
changes to the config format, actions we've taken to improve the tooling for
configuration upgrades, and additional steps we may want to consider.

## Background

A Tendermint node reads configuration settings at startup from a TOML formatted
text file, typically named `config.toml`. The contents of this file are defined
by the [`github.com/tendermint/tendermint/config`][config-pkg].

Although many settings in this file remain valid from one version of Tendermint
to the next, new versions of Tendermint often add, update, and remove settings.
These changes often require manual intervention by operators who are upgrading
their nodes.

I propose we should provide better tools and documentation to help operators
make configuration changes correctly during version upgrades.  Ideally, as much
as possible of any configuration file update should be automated, and where
that is not possible or practical, we should provide clear, explicit directions
for what steps need to be taken manually. Moreover, when the node discovers
incorrect or invalid configuration, we should improve the diagnostics it emits
so that the operator can quickly and easily find the relevant documentation,
without having to grep through source code.

## Discussion

By convention, we are supposed to document required changes to the config file
in the `UPGRADING.md` file for the release that introduces them.  Although we
have mostly done this, the level of detail in the upgrading instructions is
often insufficient for an operator to correctly update their file.

The updates vary widely in complexity: Operators may need to add new required
settings, update obsolete values for existing settings, move or rename existing
settings within the file, or remove obsolete settings (which are thus invalid).
Here are a few examples of each of these cases:

- **New required settings:** Tendermint v0.35 added a new top-level `mode`
  setting that determines whether a node runs as a validator, a full node, or a
  seed node.  The default value is `"full"`, which means the operator of a
  validator must manually add `mode = "validator"` (or set the `--mode` flag on
  the command line) for their node to come up in the correct mode.

- **Updated obsolete values:** Tendermint v0.35 removed support for versions
  `"v1"` and `"v2"` of the blocksync (formerly "fastsync") protocol, requiring
  any node using either of those values to update to `"v0"`.

- **Moved/renamed settings:** Version v0.34 moved the top-level `pprof_laddr`
  setting under the `[rpc]` section.

  Version v0.35 renamed every setting in the file from `snake_case` to
  `kebab-case`, moved the top-level `fast_sync` setting into the `[blocksync]`
  section as (itself renamed from `[fastsync]`), and moved all the top-level
  `priv-validator-*` settings under a new `[priv-validator]` section with their
  prefix trimmed off.

- **Removed obsolete settings:** Version v0.34 removed the `index_all_keys` and
  `index_keys` settings from the `[tx_index]` section; version v0.35 removed
  the `wal-dir` setting from the `[mempool]` section, and version v0.36 removed
  the `[blocksync]` section entirely.

While many of these changes are mentioned in the config section of the upgrade
instructions, some are not mentioned at all, or are hidden in other parts of
the doc. For instance, the v0.34 `pprof_laddr` change was documented only as an
RPC flag change. (A savvy reader might realize that the flag `--rpc.pprof_laddr`
implies a corresponding config section, but it omits the related detail that
there was a top-level setting that's been renamed).  The lesson here is not
that the docs are bad, but to point out that prose is not the most efficient
format to convey detailed changes like this. The upgrading instructions are
still valuable for the human reader to understand what to expect.

### Concrete Steps

As part of the v0.36 development cycle, we spent some time reverse-engineering
the configuration changes since the v0.34 release and built an experimental
command-line tool called [`confix`][confix], whose job it is to automatically
update the settings in a `config.toml` file to the latest version.  We also
backported a version of this tool into the v0.35.x branch at release v0.35.4.

This tool should work fine for configuration files created by Tendermint v0.34
and later, but does not (yet) know how to handle changes from prior versions of
Tendermint. Part of the difficulty for older versions is simply logistical: To
figure out which changes to apply, we need to understand something about the
version that made the file, as well as the version we're converting it to.

> **Discussion point:** In the future we might want to consider incorporating
> this into the node CLI directly, but we're keeping it separate for now until
> we can get some feedback from operators.

For the experiment, we handled this by carefully searching the history of
config format changes for shibboleths to bound the version: For example, the
`[fastsync]` section was added in Tendermint v0.32 and renamed `[blocksync]` in
Tendermint v0.35. So if we see a `[fastsync]` section, we have some confidence
that the file was created by v0.32, v0.33, or v0.34.

But such signals are delicate: The `[blocksync]` section was removed in v0.36,
so if we do not find `[fastsync]`, we cannot conclude from that alone that the
file is from v0.31 or earlier -- we have to look for corroborating details.
While such "sniffing" tactics are fine for an experiment, they aren't as robust
as we might like.

This is especially relevant for configuration files that may have already been
manually upgraded across several versions by the time we are asked to update
them again.  Another related concern is that we'd like to make sure conversion
is idempotent, so that it would be safe to rerun the tool over an
already-converted file without breaking anything.

### Config Versioning

One obvious tactic we could use for future releases is add a version marker to
the config file. This would give tools like `confix` (and the node itself) a
way to calibrate their expectations. Rather than being a version for the file
itself, however, this version marker would indicate which version of Tendermint
is needed to read the file.

Provisionally, this might look something like:

```toml
# THe minimum version of Tendermint compatible with the contents of
# this configuration file.
config-version = 'v0.35'
```

When initializing a new node, Tendermint would populate this field with its own
version (e.g., `v0.36`). When conducting an upgrade, tools like `confix` can
then use this to decide which conversions are valid, and then update the value
accordingly. After converting a file marked `'v0.35'` to`'v0.37'`, the
conversion tool sets the file's `config-version` to reflect its compatibility.

> **Discussion point:** This example presumes we would keep config files
> compatible within a given release cycle, e.g., all of v0.36.x. We could also
> use patch numbers here, if we think there's some reason to permit changes
> that would require config file edits at that granularity. I don't think we
> should, but that's a design question to consider.

Upon seeing an up-to-date version marker, the conversion tool can simply exit
with a diagnostic like "this file is already up-to-date", rather than sniffing
the keyspace and potentially introducing errors. In addition, this would let a
tool detect config files that are _newer_ than the one it understands, and
issue a safe diagnostic rather than doing something wrong.  Plus, besides
avoiding potentially unsafe conversions, this would also serve as
human-readable documentation that the file is up-to-date for a given version.

Adding a config version would not address the problem of how to convert files
created by older versions of Tendermint, but it would at least help us build
more robust config tooling going forward.

## Research Notes

Discovering when various configuration settings were added, updated, and
removed turns out to be surprisingly tedious.  To solve this puzzle, we had to
answer the following questions:

1. What changes were made between v0.x and v0.y? This is further complicated by
   cases where we have backported config changes into the middle of an earlier
   release cycle (e.g., `psql-conn` from v0.35.x into v0.34.13).

2. When during the development cycle were those changes made? This allows us to
   recognize features that were backported into a previous release.

3. What were the defaultvalues of the changed settings, and did they change at
   all during or across the release boundary?

Each step of the [configuration update plan][plan] is commented with a link to
one or more PRs where that change was made. The sections below discuss how we
found these references.

### Tracking Changes Across Releases

To figure out what changed between two releases, we built a tool called
[`condiff`][condiff], which performs a "keyspace" diff of two TOML documents.
This diff respects the structure of the TOML file, but ignores comments, blank
lines, and configuration values, so that we can see what was added and removed.

To use it, run:

```shell
go run ./scripts/confix/condiff old.toml new.toml
```

This tool works on any TOML documents, but for our purposes we needed
Tendermint `config.toml` files. The easiest way to get these is to build the
node binary for your version of interest, run `tendermint init` on a clean home
directory, and copy the generated config file out. The [`testdata`][testdata]
directory for the `confix` tool has configs generated from the heads of each
release branch from v0.31 through v0.35.

If you want to reproduce this yourself, it looks something like this:

```shell
# Example for Tendermint v0.32.
git checkout --track origin/v0.32.x
go get golang.org/x/sys/unix
go mod tidy
make build
rm -fr -- tmhome
./build/tendermint --home=tmhome init
cp tmhome/config/config.toml config-v32.toml
```

Be advised that the further back you go, the more idiosyncrasies you will
encounter. For example, Tendermint v0.31 and earlier predate Go modules (v0.31
used dep), and lack backport branches. And you may need to do some editing of
Makefile rules once you get back into the 20s.

Note that when diffing config files across the v0.34/v0.35 gap, the swap from
`snake_case` to `kebab-case` makes it look like everything changed. The
`condiff` tool has a `-desnake` flag that normalizes all the keys to kebab case
in both inputs before comparison.

### Locating Additions and Deletions

To figure out when a configuration setting was added or removed, your tool of
choice is `git bisect`. The only tricky part is finding the endpoints for the
search.  If the transition happened within a release, you can use that
release's backport branch as the endpoint (if it has one, e.g., `v0.35.x`).

However, the start point can be more problematic. The backport branches are not
ancestors of `master` or of each other, which means you need to find some point
in history _prior_ to the change but still attached to the mainline. For recent
releases there is a dev root (e.g., `v0.35.0-dev`, `v0.34.0-dev1`, etc.). These
are not named consistently, but you can usually grep the output of `git tag` to
find them.

In the worst case you could try starting from the root commit of the repo, but
that turns out not to work in all cases. We've done some branching shenanigans
over the years that mean the root is not a direct ancestor of all our release
branches. When you find this you will probably swear a lot. I did.

Once you have a start and end point (say, `v0.35.0-dev` and `master`), you can
bisect in the usual way. I use `git grep` on the `config` directory to check
whether the case I am looking for is present. For example, to find when the
`[fastsync]` section was removed:

```shell
# Setup:
git checkout master
git bisect start
git bisect bad                 # it's not present on tip of master.
git bisect good v0.34.0-dev1   # it was present at the start of v0.34.

# Now repeat this until it gives ou a specific commit:
if git grep -q '\[fastsync\]' config ; then git bisect good ; else git bisect bad ; fi
```

The above example finds where a config was removed: To find where a setting was
added, do the same thing except reverse the sense of the test (`if ! git grep -q
...`).

## References

- [Tendermint `config` package][config-pkg]
- [`confix` command-line tool][confix]
- [`condiff` command-line tool][condiff]
- [Configuration update plan][plan]

[config-pkg]: https://godoc.org/github.com/tendermint/tendermint/config
[confix]: https://github.com/tendermint/tendermint/blob/master/scripts/confix
[condiff]: https://github.com/tendermint/tendermint/blob/master/scripts/confix/condiff
[plan]: https://github.com/tendermint/tendermint/blob/master/scripts/confix/plan.go
[testdata]: https://github.com/tendermint/tendermint/blob/master/scripts/confix/testdata
