module.exports = {
  theme: 'cosmos',
  title: 'Tendermint Core',
  base: process.env.VUEPRESS_BASE,
  themeConfig: {
    repo: 'tendermint/tendermint',
    docsRepo: 'tendermint/tendermint',
    docsDir: 'docs',
    editLinks: true,
    label: 'core',
    algolia: {
      id: "BH4D9OD16A",
      key: "59f0e2deb984aa9cdf2b3a5fd24ac501",
      index: "tendermint"
    },
    versions: [
      {
        "label": "v0.34 (latest)",
        "key": "v0.34"
      },
      {
        "label": "v0.33",
        "key": "v0.33"
      }
    ],
    topbar: {
      banner: false,
    },
    sidebar: {
      auto: true,
      nav: [
        {
          title: 'Resources',
          children: [
            {
              title: 'RPC',
              path: (process.env.VUEPRESS_BASE ? process.env.VUEPRESS_BASE : '/')+'rpc/',
              static: true
            },
          ]
        }
      ]
    },
    gutter: {
      title: 'Help & Support',
      editLink: true,
      forum: {
        title: 'Tendermint Discussions',
        text: 'Join the Tendermint discussions to learn more',
        url: 'https://github.com/tendermint/tendermint/discussions',
        bg: '#0B7E0B',
        logo: 'tendermint'
      },
      github: {
        title: 'Found an Issue?',
        text: 'Help us improve this page by suggesting edits on GitHub.'
      }
    },
    footer: {
      question: {
        text: 'Chat with Tendermint developers in <a href=\'https://discord.gg/vcExX9T\' target=\'_blank\'>Discord</a> or reach out on <a href=\'https://github.com/tendermint/tendermint/discussions\' target=\'_blank\'>GitHub</a> to learn more.'
      },
      logo: '/logo-bw.svg',
      textLink: {
        text: 'tendermint.com',
        url: 'https://tendermint.com'
      },
      services: [
        {
          service: 'medium',
          url: 'https://medium.com/@tendermint'
        },
        {
          service: 'twitter',
          url: 'https://twitter.com/tendermint_team'
        },
        {
          service: 'linkedin',
          url: 'https://www.linkedin.com/company/tendermint/'
        },
        {
          service: 'reddit',
          url: 'https://reddit.com/r/cosmosnetwork'
        },
        {
          service: 'telegram',
          url: 'https://t.me/cosmosproject'
        },
        {
          service: 'youtube',
          url: 'https://www.youtube.com/c/CosmosProject'
        }
      ],
      smallprint:
        'The development of Tendermint Core is led primarily by [Interchain GmbH](https://interchain.berlin/). Funding for this development comes primarily from the Interchain Foundation, a Swiss non-profit. The Tendermint trademark is owned by Tendermint Inc, the for-profit entity that also maintains this website.',
      links: [
        {
          title: 'Documentation',
          children: [
            {
              title: 'Cosmos SDK',
              url: 'https://docs.cosmos.network'
            },
            {
              title: 'Cosmos Hub',
              url: 'https://hub.cosmos.network'
            }
          ]
        },
        {
          title: 'Community',
          children: [
            {
              title: 'Tendermint blog',
              url: 'https://medium.com/@tendermint'
            },
            {
              title: 'GitHub Discussions',
              url: 'https://github.com/tendermint/tendermint/discussions'
            }
          ]
        },
        {
          title: 'Contributing',
          children: [
            {
              title: 'Contributing to the docs',
              url: 'https://github.com/tendermint/tendermint'
            },
            {
              title: 'Source code on GitHub',
              url: 'https://github.com/tendermint/tendermint'
            },
            {
              title: 'Careers at Tendermint',
              url: 'https://tendermint.com/careers'
            }
          ]
        }
      ]
    }
  },
  plugins: [
    [
      '@vuepress/google-analytics',
      {
        ga: 'UA-51029217-11'
      }
    ],
    [
      '@vuepress/plugin-html-redirect',
      {
        countdown: 0
      }
    ]
  ]
};
