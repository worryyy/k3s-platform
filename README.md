k3s-platform/
├── ansible.cfg
├── inventory/
│   └── dev.ini
├── group_vars/
│   └── all.yml
├── playbooks/
│   ├── site.yml
│   ├── bootstrap.yml
│   ├── k3s-cluster.yml
│   ├── kubeconfig.yml
│   ├── verify.yml
│   └── app-deploy.yml
├── roles/
│   ├── common/
│   ├── k3s_server/
│   ├── k3s_agent/
│   ├── kubeconfig/
│   └── kube_verify/
├── apps/
│   └── sre-demo-api/
│       ├── Dockerfile
│       ├── .dockerignore
│       ├── go.mod
│       ├── go.sum
│       ├── cmd/
│       │   └── server/
│       │       └── main.go
│       └── internal/
│           ├── handler/
│           ├── metrics/
│           └── service/
├── manifests/
│   └── app/
│       ├── namespace.yaml
│       ├── deployment.yaml
│       ├── service.yaml
│       ├── servicemonitor.yaml
│       └── kustomization.yaml
├── helm-values/
│   └── .gitkeep
└── README.md