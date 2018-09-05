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
          "/tendermint-core/how-to-read-logs",
          "/tendermint-core/block-structure",
          "/tendermint-core/light-client-protocol",
          "/tendermint-core/metrics",
          "/tendermint-core/secure-p2p",
          "/tendermint-core/validators"
        ]
      },
      {
        title: "Tendermint Tools",
        collapsable: false,
        children: ["tools/benchmarking", "tools/monitoring"]
      },
      {
        title: "Tendermint Networks",
        collapsable: false,
        children: [
          "/networks/deploy-testnets",
          "/networks/terraform-and-ansible",
          "/networks/fast-sync"
        ]
      },
      {
        title: "Application Development",
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
        title: "Research",
        collapsable: false,
        children: ["/research/determinism", "/research/transactional-semantics"]
      }
    ]
  }
};
