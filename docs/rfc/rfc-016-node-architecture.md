# RFC 016: Node Architecture

## Changelog

- April 6, 2022: Initial draft (@cmwaters)

## Abstract

The node package is arguable the entry point into the Tendemrint codebase but

## Background

> Any context or orientation needed for a reader to understand and participate
> in the substance of the Discussion. If necessary, this section may include
> links to other documentation or sources rather than restating existing
> material, but should provide enough detail that the reader can tell what they
> need to read to be up-to-date.

### References

> Links to external materials needed to follow the discussion may be added here.
>
> In addition, if the discussion in a request for comments leads to any design
> decisions, it may be helpful to add links to the ADR documents here after the
> discussion has settled.

## Discussion

Points of discussion:

- Block executor should be the sole writer to the block and state store. The reactors Consensus, BlockSync and StateSync should all import the executor for advancing state.

> This section contains the core of the discussion.
>
> There is no fixed format for this section, but ideally changes to this
> section should be updated before merging to reflect any discussion that took
> place on the PR that made those changes.
