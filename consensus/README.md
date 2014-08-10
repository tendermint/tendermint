## [ZombieValidators]

The most likely scenario may be during an upgrade.


We'll see some validators that fail to upgrade while most have.  Then, some updated validator will propose a block that appears invalid to the outdated validators.  What is the outdated validator to do?


The right thing to do is to stop participating, because you have no idea what is going on, and prompt the administrator to upgrade the daemon.  (Now this could be a security issue if not handled properly, so in the future we should think about upgrade security best practices).  Yet say you don't, and you continue to sign blocks without really participating in the consensus rounds -- maybe voting nil each time and then signing whatever is decided on.  Well for one, you've lost all ability to validate any blocks.  It's a problem because if there are too many of these zombies, the network might accidentally commit a bad block -- in effect, crashing the network.  So, the network wants to weed the zombies out.


It's hard catching the zombie.  It can mimick whatever other validator is doing, perhaps mimicking the first one to vote during the rounds and waiting just a little longer for the final block vote.  Based on my extensive experience with zombie flicks, it appears that the best way to identify a zombie is to make it perform some action that only non-zombies would understand.  That's easy! Just make each version of the protocol have a special algorithm that selects a small but sufficiently large fraction of the validator population at each block, and make them perform an action (intuitively, make them raise their hadns).  Eventually, it'll become the zombie's turn to do something but it won't know what to do.  Or it will act out of turn.  Gotcha.


The algorithm could even depend on state data, such that validators are required to keep it updated, which is a hair away from full validation.  I suspect that there are more complete ways to enforce validation, but such measures may not be necessary in practice.

TODO: implement such a mechanism.
