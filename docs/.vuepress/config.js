module.exports = {
  title: "Tendermint Documentation",
  description: "Documentation for Tendermint Core.",
  ga: "UA-51029217-1",
  dest: "./dist/docs",
  base: "/docs/",
  markdown: {
    lineNumbers: true
  },
  themeConfig: {
    repo: "tendermint/tendermint",
    editLinks: true,
    docsDir: "docs",
    docsBranch: "develop",
    editLinkText: 'Edit this page on Github',
    lastUpdated: true,
    algolia: {
      apiKey: '59f0e2deb984aa9cdf2b3a5fd24ac501',
      indexName: 'tendermint',
      debug: false
    },
    nav: [
      { text: "Back to Tendermint", link: "https://tendermint.com" },
      { text: "RPC", link: "https://tendermint.com/rpc/" }
    ],
    sidebar: [
      {
        title: "Introduction",
        collapsable: false,
        children: [
          "/introduction/",
          "/introduction/quick-start",
          "/introduction/install",
          "/introduction/what-is-tendermint"
        ]
      },
      {
        title: "Guides",
        collapsable: false,
        children: [
          "/guides/go-built-in",
          "/guides/go"
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
          "/spec/abci/abci",
          "/app-dev/ecosystem"
        ]
      },
      {
        title: "Tendermint Core",
        collapsable: false,
        children: [
          "/tendermint-core/",
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
          "/tendermint-core/validators",
          "/tendermint-core/mempool"
        ]
      },
      {
        title: "Networks",
        collapsable: false,
        children: [
          "/networks/",
          "/networks/docker-compose",
          "/networks/terraform-and-ansible",
        ]
      },
      {
        title: "Tools",
        collapsable: false,
        children:  [
          "/tools/",
          "/tools/benchmarking",
          "/tools/monitoring",
          "/tools/remote-signer-validation"
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
        title: "ABCI Spec",
        collapsable: false,
        children: [
          "/spec/abci/",
          "/spec/abci/abci",
          "/spec/abci/apps",
          "/spec/abci/client-server"
        ]
      }
    ]
  }
};
