workspace/
├── platform/
├── server/
│   ├── Dockerfile
│   ├── go.mod
│   ├── go.sum
│   ├── cmd/
│   ├── internal/
│   │
│   ├── charts/
│   │   └── platform-server/
│   │       ├── Chart.yaml
│   │       ├── values.yaml
│   │       └── templates/
│   │           ├── api-deployment.yaml
│   │           ├── api-service.yaml
│   │           ├── worker-deployment.yaml
│   │           ├── configmap.yaml
│   │           ├── serviceaccount.yaml
│   │           ├── role.yaml
│   │           ├── rolebinding.yaml
│   │           └── _helpers.tpl
│   │
│   ├── config/
│   │   └── service-catalog.example.yaml
│   │
│   └── ci/
│       └── Jenkinsfile
│
└── k3s/
    ├── ansible.cfg
    ├── inventory/
    ├── playbooks/
    ├── roles/
    │
    ├── helm-values/
    │   └── platform/
    │       ├── platform-server-dev.yaml
    │       ├── postgresql.yaml
    │       ├── rabbitmq.yaml
    │       └── service-catalog.yaml
    │
    ├── gitops/
    │   └── applications/
    │       └── platform/
    │           ├── platform-server.yaml
    │           ├── platform-postgresql.yaml
    │           └── platform-rabbitmq.yaml
    │
    ├── secrets/
    │   ├── platform-server-secrets.example.yaml
    │   ├── platform-server-secrets.yaml
    │   ├── postgresql-auth.example.yaml
    │   ├── rabbitmq-auth.example.yaml
    │   └── tcr-secret.example.yaml
    │
    └── scripts/
        └── create-pull-secrets.sh