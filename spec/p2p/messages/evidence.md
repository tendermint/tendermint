---
order: 3
---

# Evidence

## Channel

Evidence has one channel. The channel identifier is listed below.

| Name            | Number |
|-----------------|--------|
| EvidenceChannel | 56     |

## Message Types

### Evidence

Verified evidence that has already been propagated throughout the network. This evidence will appear within the EvidenceList struct of a [block](../../core/data_structures.md#block) as well.

| Name     | Type                                                        | Description            | Field Number |
|----------|-------------------------------------------------------------|------------------------|--------------|
| evidence | [Evidence](../../core/data_structures.md#evidence) | Valid evidence | 1            |
