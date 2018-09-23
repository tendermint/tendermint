module.exports = {
  title: "Tendermint Core",
  description: "Documentation for Tendermint Core",
  dest: "./dist/docs",
  base: "/docs/",
  markdown: {
    lineNumbers: true
  },
  themeConfig: {
    lastUpdated: "Last Updated",
    nav: [{ text: "Back to Tendermint", link: "https://tendermint.com" }],
    sidebar: [
      {
        title: "Getting Started",
        collapsable: false,
        children: [
          "/introduction/quick-start",
          "/introduction/install",
          "/introduction/introduction"
        ]
      },
      {
        title: "Tendermint Core",
        collapsable: false,
        children: [
          "/tendermint-core/using-tendermint",
          "/tendermint-core/configuration",
          "/tendermint-core/rpc",
          "/tendermint-core/running-in-production",
          "/tendermint-core/fast-sync",
          "/tendermint-core/how-to-read-logs",
          "/tendermint-core/block-structure",
          "/tendermint-core/light-client-protocol",
          "/tendermint-core/metrics",
          "/tendermint-core/secure-p2p",
          "/tendermint-core/validators"
        ]
      },
      {
        title: "Tools",
        collapsable: false,
        children:  [
	  "tools/benchmarking",
	  "tools/monitoring"
	]
      },
      {
        title: "Networks",
        collapsable: false,
        children: [
          "/networks/deploy-testnets",
          "/networks/terraform-and-ansible",
        ]
      },
      {
        title: "Apps",
        collapsable: false,
        children: [
          "/app-dev/getting-started",
          "/app-dev/abci-cli",
          "/app-dev/app-architecture",
          "/app-dev/app-development",
          "/app-dev/subscribing-to-events-via-websocket",
          "/app-dev/indexing-transactions",
          "/app-dev/abci-spec",
          "/app-dev/ecosystem"
        ]
      },
      {
        title: "Tendermint Spec",
        collapsable: true,
        children: [
          "/spec/",
          "/spec/blockchain/blockchain",
          "/spec/blockchain/encoding",
          "/spec/blockchain/state",
          "/spec/software/abci",
          "/spec/consensus/bft-time",
          "/spec/consensus/consensus",
          "/spec/consensus/light-client",
          "/spec/software/wal",
          "/spec/p2p/config",
          "/spec/p2p/connection",
          "/spec/p2p/node",
          "/spec/p2p/peer",
          "/spec/reactors/block_sync/reactor",
          "/spec/reactors/block_sync/impl",
          "/spec/reactors/consensus/consensus",
          "/spec/reactors/consensus/consensus-reactor",
          "/spec/reactors/consensus/proposer-selection",
          "/spec/reactors/evidence/reactor",
          "/spec/reactors/mempool/concurrency",
          "/spec/reactors/mempool/config",
          "/spec/reactors/mempool/functionality",
          "/spec/reactors/mempool/messages",
          "/spec/reactors/mempool/reactor",
          "/spec/reactors/pex/pex",
          "/spec/reactors/pex/reactor",
	]
      },
      {
        title: "ABCI Specification",
        collapsable: false,
        children: [
          "/spec/abci/abci",
          "/spec/abci/apps",
          "/spec/abci/client-server"
        ]
      }
    ]
  }
};
