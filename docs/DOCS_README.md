# Documentation Maintenance Overview

The documentation found in this directory is hosted at:

- https://tendermint.com/docs/

and built using [VuePress](https://vuepress.vuejs.org/) like below:

```bash
npm install -g vuepress # global install vuepress tool, only once
npm install

mkdir -p .vuepress && cp config.js .vuepress/
vuepress build
```

Under the hood, Jenkins listens for changes (on develop or master) in ./docs then rebuilds
either the staging or production site depending on which branch the changes were made.

To update the Table of Contents (layout of the documentation sidebar), edit the
`config.js` in this directory, while the `README.md` is the landing page for the
website documentation.

To view the latest documentation on the develop branch, see the staging site:

- https://tendermint-staging.interblock.io/docs/

and the documentation on master branch is found here:

- https://tendermint.com/docs/

