# `tendermint/block`

## Block

TODO: document

### Header

### Validation

### Data

## PartSet

PartSet is used to split a byteslice of data into parts (pieces) for transmission.
By splitting data into smaller parts and computing a Merkle root hash on the list,
you can verify that a part is legitimately part of the complete data, and the
part can be forwarded to other peers before all the parts are known.  In short,
it's a fast way to propagate a large file over a gossip network.

PartSet was inspired by the LibSwift project.

Usage:

```Go
data := RandBytes(2 << 20) // Something large

partSet := NewPartSetFromData(data)
partSet.Total()     // Total number of 4KB parts
partSet.Count()     // Equal to the Total, since we already have all the parts
partSet.Hash()      // The Merkle root hash
partSet.BitArray()  // A BitArray of partSet.Total() 1's

header := partSet.Header() // Send this to the peer
header.Total        // Total number of parts
header.Hash         // The merkle root hash

// Now we'll reconstruct the data from the parts
partSet2 := NewPartSetFromHeader(header)
partSet2.Total()    // Same total as partSet.Total()
partSet2.Count()    // Zero, since this PartSet doesn't have any parts yet.
partSet2.Hash()     // Same hash as in partSet.Hash()
partSet2.BitArray() // A BitArray of partSet.Total() 0's

// In a gossip network the parts would arrive in arbitrary order, perhaps
// in response to explicit requests for parts, or optimistically in response
// to the receiving peer's partSet.BitArray().
for !partSet2.IsComplete() {
    part := receivePartFromGossipNetwork()
    added, err := partSet2.AddPart(part)
    if err != nil {
		// A wrong part,
        // the merkle trail does not hash to partSet2.Hash()
    } else if !added {
        // A duplicate part already received
    }
}

data2, _ := ioutil.ReadAll(partSet2.GetReader())
bytes.Equal(data, data2) // true
```
