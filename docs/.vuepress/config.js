module.exports = {
  theme: "cosmos",
  themeConfig: {
    docsRepo: "tendermint/tendermint",
    editLinks: true,
    docsDir: "docs",
    logo: "/logo.svg",
    label: "core",
    sidebar: [
      {
        title: "Resources",
        children: [
          {
            title: "Developer Sessions",
            path: "/DEV_SESSIONS"
          },
          {
            title: "RPC",
            static: true,
            path: "/rpc/"
          }
        ]
      }
    ]
  },
  markdown: {
    anchor: {
      permalinkSymbol: ""
    }
  }
};
