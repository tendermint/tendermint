package version

const Maj = "0"
const Min = "7" // tmsp useability (protobuf, unix); optimizations; broadcast_tx_commit
const Fix = "3" // fixes to event safety, mempool deadlock, hvs race, replay non-empty blocks

const Version = Maj + "." + Min + "." + Fix
