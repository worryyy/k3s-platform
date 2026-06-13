k3s/
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