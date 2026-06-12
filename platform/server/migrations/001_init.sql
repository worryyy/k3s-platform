create table if not exists releases (
  id text primary key,

  service_name text not null,
  environment text not null,
  branch text not null,

  status text not null,
  operator text not null,

  jenkins_job text,
  jenkins_build_number integer,

  commit_sha text,
  image_repo text,
  image_tag text,
  image_digest text,

  argocd_app text,
  namespace text,
  deployment text,

  error_message text,

  started_at timestamptz,
  finished_at timestamptz,

  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists release_events (
  id bigserial primary key,
  release_id text not null references releases(id),

  status text not null,
  message text not null,
  detail jsonb,

  created_at timestamptz not null default now()
);

create table if not exists release_locks (
  service_name text not null,
  environment text not null,

  release_id text not null,
  locked_until timestamptz not null,

  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),

  primary key (service_name, environment)
);
