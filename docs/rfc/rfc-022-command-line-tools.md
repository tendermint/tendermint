# RFC 022: Tendermint Command Line Tools

## Changelog

- 24-May-2022: Initial draft (@creachadair)

## Abstract

The stock `tendermint` command-line tool has a large collection of subcommands.
Some are essential to the operation of the consensus node, others are ancillary
utilities. Unlike the core operational subcommands, many of the utilities are
_ad hoc_ and intended to address specific operational corner cases. For this
reason, the utility subcommands also have less clearly-defined behavior and are
less well-tested.  This contributes to surprising and sometimes negative user
experiences for node operators.

This RFC summarizes the existing subcommands, discusses some of the known
issues, and advances some strategies to mitigate those issues.

## Background

As of Tendermint v0.35, the stock `tendermint` command-line tool built from the
core repository (e.g., by running `make build`) exports a large collection of
subcommands. These can be roughly organized by their role:

### Core Subcommands

The subcommands in this group are core to the operation of a consensus node.

```
  gen-node-key   Generate a new node key
  gen-validator  Generate new validator keypair
  init           Initializes a Tendermint node
  light          Run a light client proxy server, verifying Tendermint rpc
  start          Run the tendermint node
  version        Show version info
```

### Introspection Subcommands

The subcommands in this group help operators inspect the state of a node, and
to diagnose issues in a running or failed node.

```
  debug          A utility to kill or watch a Tendermint process while aggregating debugging data
  inspect        Run an inspect server for investigating Tendermint state
  show-node-id   Show this node's ID
  show-validator Show this node's validator info
```

### Database Subcommands

The subcommands in this group help operators recover from specific node and/or
application failures by reading and manipulating a node's underlying databases.

```
  reindex-event  reindex events to the event store backends
  replay         Replay messages from WAL
  replay-console Replay messages from WAL in a console
  reset          Set of commands to conveniently reset tendermint related data
  rollback       rollback tendermint state by one height
```

### Auxiliary Subcommands

Subcommands in this group are general support utilities that do not fit into
the other categories.

```
  key-migrate    Run Database key migration
  testnet        Initialize files for a Tendermint testnet
```

## Discussion

Over time, the `tendermint` command-line tool has accumulated a substantial
grab-bag of utility subcommands.  In addition to presenting a large API
surface, many of the CLI subcommands do not hae well-defined semantics, and
some can be easily misused to the operator's detriment.

A notable recent example was `unsafe-reset-all` (moved in v0.36 under a new
`reset` command group) which destroys all the node's databases and its
validator key settings without confirmation. While that's its intended
function, that is rarely what an operator really wants.

Besides being tricky to use correctly, utility subcommands may interact in
unpredictable ways with changes across different releases because of other
internal plumbing changes.  For example, Tendermint v0.35.x includes changes to
the way the node configuration file is discovered, and that change affects some
of the utility subcommands. We verified they worked for v0.35.x, but when we
backported some subcommand changes into v0.34.x, there were some hidden
dependencies that weren't included, leading to [#8369][issue8369].

Historically, the argument for including these subcommands into the node binary
was to avoid making node operators download a lot of individual separate tools
for important but specialized use cases. This is a reasonable goal, but should
also be balanced against the goal of having a clear, robust, well-defined, and
reliable command-line API for the consensus node.

In light of the above, I propose we should take a more deliberate approach to
the design of the `tendermint` command-line API, one that makes a more clear
separation between essential functionality and specialized utilities. The
sections below discuss two specific approaches we could take.

### Approach 1: Command Groups

One way to improve the API is to make more extensive use of command "groups".
We have already done this to some extent in v0.36 with the `reset` group, which
collects `reset-state`, `unsafe-reset-all`, and `unsafe-reset-priv-validator`
under a single heading.

Advantages:

- Grouping "dangerous" or special-purpose commands under a heading communicates
  intent more clearly.

- Adding additional groups is relatively easy to do, and operators would have
  to update their scripts but would not need to download or install any new
  tools.

Disadvantages:

- Fixes to utility commands are still connected tightly to Tendermint releases,
  making it difficult to hotfix subcommand bugs and provide quick remediation
  for issues that operators are facing without tagging a whole new release.
  This leads to new releases including half-baked CLI addons that wind up as a
  maintenance headache for both the TM Core team and operators later on.

- Libraries like the SDK and tools like `gaiad` that wrap Tendermint commands
  can easily export specialized, dangerous, and potentially non-production
  ready functionality to higher-level consumers without good signaling.

### Approach 2: Utility Tool

Another approach would be to split some of the "utility" subcommands into a
separate tool. Rather than having each one become its own binary, they would be
moved to a dedicated `tmutil` program.  Like the main `tendermint` binary, this
tool could use a subcommand structure so that operators would not need to
download many different programs, but doing this would isolate the specialized,
dangerous, and non-production ready code from the main Tendermint release.

Advantages:

- A separate `tmutil` program could be released on a separate schedule from the
  main Tendermint core, allowing us to quickly provide helper tools to
  operators who are in a jam, and to hotfix issues in utility subcommands
  without triggering a whole Tendermint release process.

- Separating non-essential utilities from the core makes it much more clear
  when the user is doing something potentially dangerous.

- Programs that "wrap" Tendermint (such as the SDK and `gaiad`) would have to
  make a deliberate extra effort to depend on the internal details we use to
  make utility subcommands work, and to expose risky low-level functionality to
  end users.

  Many of those surfaces should not be considered part of the public API of
  Tendermint, and consumers should have to be explicit about building on top of
  them.

Disadvantages:

- Operators would need to download or install a separate tool for utility
  subcommands (though it need not be many different programs).

### Summary

The options discussed above are not the only possibilities, but those are a
couple of plausible approaches. Other suggestions are welcome. The main claim
here is that we should be doing _something_ differently, and to advance some
paths forward.

We do not have to choose between having a robust, reliable production node and
delivering useful tools to application developers and node operators.  The
situation we have now is harmful to both: Coupling utilities with the core
makes it harder and slower to ship support code, exposes users to friction when
it changes, and makes it difficult for the core team to make improvements.

We ought to adopt balanced separation of these concerns so that operators can
still get the tools they need without too much friction, but without putting
the velocity and stability of the core at risk.

## References

- [#8369: priv_validator-state.json does not get regenerated][issue8369]

[issue8369]: https://github.com/tendermint/tendermint/issues/8389
