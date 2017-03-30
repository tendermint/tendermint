# Setup a Kubernetes cluster on Digital Ocean (DO)

Available options:

1. [kubeadm (alpha)](https://kubernetes.io/docs/getting-started-guides/kubeadm/)
2. [kargo](https://kubernetes.io/docs/getting-started-guides/kargo/)
3. [rancher](http://rancher.com/)
4. [terraform](https://github.com/hermanjunge/kubernetes-digitalocean-terraform)

As you can see, there is no single tool for creating a cluster on DO.
Therefore, choose the one you know and comfortable working with. If you know
and used [terraform](https://www.terraform.io/) before, then choose it. If you
know Ansible, then pick kargo. If none of these seem familiar to you, go with
kubeadm. Rancher is a beautiful UI for deploying and managing containers in
production.
