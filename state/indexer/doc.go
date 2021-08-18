/*
Package indexer defines Tendermint's block and transaction event indexing logic.

Currently, Tendermint supports two primary means of block and transaction event
indexing:

1. A key-value sink via an embedded database with a proprietary query language.
2. A Postgres-based sink.

Recall, an ABCI application can emit events during block and transaction execution
in the form of <abci.Event.Type>.<abci.EventAttributeKey>.<abci.EventAttributeValue>,
e.g. 'transfer.amount=10000'.

An operator can enable one or both of the supported indexing sinks via the
'tx_index.indexer' Tendermint configuration.

Example:

	[tx-index]
	indexer = ["kv", "psql"]

If an operator wants to completely disable indexing, they may simply just provide
the "null" sink option in the configuration. All other sinks will be ignored if
"null" is provided.

If indexing is enabled, index.Service will iterate over all enabled sinks and
invoke block and transaction indexing via the appropriate IndexBlockEvents and
IndexTxEvents methods.

Note, the "kv" sink is considered deprecated and it's query functionality is very
limited, but does allow users to directly query for block and transaction events
against Tendermint's RPC. Instead, operators are encouraged to instead use the
"psql" indexing sink when more complex queries are required and for reliability
purposes as PostgreSQL can scale.

Prior to starting Tendermint with the "psql" indexing sink enabled, operators
must ensure the following:

1. The "psql" indexing sink is provided in Tendermint's configuration.
2. A 'tx-index.psql-conn' value is provided that contains the PostgreSQL connection URI.
3. The block and transaction event schemas have been created in the PostgreSQL database.

Tendermint provides the block and transaction event schemas in the following
path: state/indexer/sink/psql/schema.sql

To create the schema in a PostgreSQL database, operators can perform the query
with the schema manually or invoke schema creation via the CLI:

	$ psql <flags> -f state/indexer/sink/psql/schema.sql

Note, using the "psql" indexing sink prohibits queries against the RPC. However,
this is due to the fact that querying can and should be done directly against
the PostgreSQL database instead as it offers a rich query language and features.

Operators are able to perform queries such as the following:

* Query for all transaction events for a given transaction hash:

  SELECT * FROM tx_events WHERE hash = '3E7D1F...';

* Query for all transaction events for a given block height:

	SELECT * FROM tx_events WHERE height = 25;

* Query for transaction events that have a given type (i.e. value wildcard):

	SELECT * FROM tx_events WHERE key LIKE '%transfer.recipient%';


Note, if the entire abci.TxResult is needed, a foreign key exists in the tx_events
table that maps to tx_results. The tx_results table contains the raw Protobuf
encoded abci.TxResult object.
*/
package indexer
