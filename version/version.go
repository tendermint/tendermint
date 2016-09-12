package version

const Maj = "0"
const Min = "7" // tmsp useability (protobuf, unix); optimizations; broadcast_tx_commit
const Fix = "2" // query conn, peer filter, fast sync fix (+hot fix to tmsp connecting)

const Version = Maj + "." + Min + "." + Fix
