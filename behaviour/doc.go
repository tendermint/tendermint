/*
Package Behaviour provides a mechanism for reactors to report behaviour of peers.

Instead of a reactor calling the switch directly it will call the behaviour module which will
handle the stoping and marking peer as good on behalf of the reactor.

There are four different behaviours a reactor can report.

1. bad message

	type badMessage struct {
		explanation string
	}

# This message will request the peer be stopped for an error

2. message out of order

	type messageOutOfOrder struct {
		explanation string
	}

# This message will request the peer be stopped for an error

3. consesnsus Vote

	type consensusVote struct {
		explanation string
	}

# This message will request the peer be marked as good

4. block part

	type blockPart struct {
		explanation string
	}

This message will request the peer be marked as good
*/
package behaviour
