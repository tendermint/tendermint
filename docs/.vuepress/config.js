module.exports = {
  theme: "cosmos",
  // locales: {
  //   "/": {
  //     lang: "en-US"
  //   },
  //   "/ru/": {
  //     lang: "ru"
  //   }
  // },
  themeConfig: {
    docsRepo: "tendermint/tendermint",
    editLink: true,
    docsDir: "docs",
    logo: "/logo.svg",
    label: "core",
    gutter: {
      title: "Help & Support",
      editLink: true,
      children: [
        {
          title: "Riot Chat",
          text: "Chat with Tendermint developers on Riot Chat.",
          highlighted: "500+ people chatting now"
        },
        {
          title: "Tendermint Forum",
          text: "Found an Issue?",
          highlighted:
            "Help us improve this page by suggesting edits on GitHub."
        }
      ]
    },
    footer: {
      logo: "/logo-bw.svg",
      textLink: {
        text: "tendermint.com",
        url: "https://tendermint.com"
      },
      services: [
        {
          service: "medium",
          url: "https://medium.com/@tendermint"
        },
        {
          service: "twitter",
          url: "https://twitter.com/tendermint_team"
        },
        {
          service: "linkedin",
          url: "https://www.linkedin.com/company/tendermint/"
        },
        {
          service: "reddit",
          url: "https://reddit.com/r/cosmosnetwork"
        },
        {
          service: "telegram",
          url: "https://t.me/cosmosproject"
        },
        {
          service: "youtube",
          url: "https://www.youtube.com/c/CosmosProject"
        }
      ],
      smallprint:
        "The development of the Tendermint project is led primarily by Tendermint Inc., the for-profit entity which also maintains this website. Funding for this development comes primarily from the Interchain Foundation, a Swiss non-profit.",
      links: [
        {
          title: "Documentation",
          children: [
            {
              title: "Cosmos SDK",
              url: "https://cosmos.network/docs"
            },
            {
              title: "Cosmos Hub",
              url: "https://hub.cosmos.network/"
            }
          ]
        },
        {
          title: "Community",
          children: [
            {
              title: "Tendermint blog",
              url: "https://medium.com/@tendermint"
            },
            {
              title: "Forum",
              url: "https://forum.cosmos.network/c/tendermint"
            },
            {
              title: "Chat",
              url: "https://riot.im/app/#/room/#tendermint:matrix.org"
            }
          ]
        },
        {
          title: "Contributing",
          children: [
            {
              title: "Contributing to the docs",
              url: "https://github.com/tendermint/tendermint"
            },
            {
              title: "Source code on GitHub",
              url: "https://github.com/tendermint/tendermint"
            },
            {
              title: "Careers at Tendermint",
              url: "https://tendermint.com/careers"
            }
          ]
        }
      ]
    },
    sidebar: [
      {
        title: "Resources",
        children: [
          {
            title: "Developer Sessions",
            path: "/DEV_SESSIONS.html"
          },
          {
            title: "RPC",
            path: "/rpc/",
            static: true
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
