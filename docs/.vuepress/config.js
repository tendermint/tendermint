module.exports = {
  theme: 'cosmos',
  title: 'Tenderdash',
  // locales: {
  //   "/": {
  //     lang: "en-US"
  //   },
  //   "/ru/": {
  //     lang: "ru"
  //   }
  // },
  base: process.env.VUEPRESS_BASE,
  themeConfig: {
    repo: 'dashevo/tenderdash',
    docsRepo: 'dashevo/tenderdash',
    docsDir: "docs",
    editLinks: true,
    label: 'core',
    algolia: {
      id: "BH4D9OD16A",
      key: "59f0e2deb984aa9cdf2b3a5fd24ac501",
      index: "tenderdash"
    },
    versions: [
      {
        "label": "v0.7",
        "key": "v0.7"
      },
      {
        "label": "v0.8",
        "key": "v0.8"
      },
      {
        "label": "master",
        "key": "master"
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
              title: 'Developer Sessions',
              path: '/DEV_SESSIONS.html'
            },
            {
              title: 'RPC',
              path: 'https://docs.tendermint.com/master/rpc/',
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
        title: 'Tenderdash Forum',
        text: 'Join the Tenderdash forum to learn more',
        url: 'https://forum.cosmos.network/c/tendermint',
        bg: '#0B7E0B',
        logo: 'tenderdash'
      },
      github: {
        title: 'Found an Issue?',
        text: 'Help us improve this page by suggesting edits on GitHub.'
      }
    },
    footer: {
      question: {
        text: 'Chat with Tenderdash developers in <a href=\'https://discord.gg/fqfCb4fX\' target=\'_blank\'>Discord</a>'
      },
      logo: '/logo-bw.svg',
      textLink: {
        text: 'dash.org',
        url: 'https://dash.org'
      },
      services: [
        {
          service: 'medium',
          url: 'https://medium.com/@dashpay'
        },
        {
          service: 'twitter',
          url: 'https://twitter.com/dashpay'
        },
        {
          service: 'linkedin',
          url: 'https://www.linkedin.com/company/dash-core-group/'
        },
        {
          service: 'reddit',
          url: 'https://reddit.com/r/dashpay'
        },
        {
          service: 'telegram',
          url: 'https://t.me/dash_chat'
        },
        {
          service: 'youtube',
          url: 'https://www.youtube.com/channel/UCAzD2v9Yx4a4iS2_-unODkA'
        }
      ],
      smallprint:
        'The development of Tenderdash is led by [Dash Core Group](https://dash.org/). Funding for this development comes primarily from the Dash Governance system. The Tendermint trademark is owned by Tendermint Inc. The Tenderdash logo is owned by Dash Core Group Inc., the company that maintains this website.',
      links: [
        {
          title: 'Documentation',
          children: [
            {
              title: 'Dash developer docs',
              url: 'https://dashplatform.readme.io/'
            },
            {
              title: 'Dash user docs',
              url: 'https://docs.dash.org/en/stable/'
            }
          ]
        },
        {
          title: 'Community',
          children: [
            {
              title: 'Dash blog',
              url: 'https://medium.com/@dashpay'
            },
            {
              title: 'Forum',
              url: 'https://forum.dash.org'
            }
          ]
        },
        {
          title: 'Contributing',
          children: [
            {
              title: 'Contributing to the docs',
              url: 'https://github.com/dashevo/tenderdash'
            },
            {
              title: 'Source code on GitHub',
              url: 'https://github.com/dashevo/tenderdash'
            },
            {
              title: 'Careers at Dash Core Group',
              url: 'https://dash,org/dcg/jobs/'
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
    ]
  ]
};
