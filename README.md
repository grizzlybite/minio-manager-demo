# minio-manager

Declarative CLI for managing MinIO **buckets, users and IAM policies** from a
single YAML file. The config is the source of truth: the tool reads the live
state of a MinIO cluster, computes a diff and applies only the necessary
changes — designed to run in CI/CD.

```
minio-manager validate   # check config syntax + semantics (no MinIO access)
minio-manager plan       # show the diff against the live cluster (dry-run)
minio-manager apply      # converge the cluster to the config
minio-manager apply --prune   # also delete resources absent from the config
minio-manager import HOST:PORT --user U --password P   # export live state to YAML
minio-manager version
```

## Build

```bash
make build            # -> bin/minio-manager (CGO disabled)
make build-static     # -> bin/minio-manager-linux-amd64 (fully static, no glibc)
make test             # unit tests with -race
make test-integration # integration tests (requires Docker)
make lint             # golangci-lint
make docker-build     # scratch image
```

## Configuration

See [configs/example.yaml](configs/example.yaml) for a complete annotated
example. The top-level structure is:

```yaml
minio:    { ... }     # connection settings
buckets:  [ ... ]     # desired buckets
users:    [ ... ]     # desired users
policies: [ ... ]     # desired IAM policies
```

Every list section is optional — a config with only `policies`, or only
`buckets`, is valid.

### `minio` — connection

| Field        | Required | Default | Notes |
|--------------|----------|---------|-------|
| `endpoint`   | for `plan`/`apply` | — | `host:port`, no scheme. Override with `--minio-endpoint` / `MINIO_ENDPOINT`. |
| `access_key` | for `plan`/`apply` | — | Override with `--minio-access-key` / `MINIO_ACCESS_KEY`. |
| `secret_key` | for `plan`/`apply` | — | Override with `--minio-secret-key` / `MINIO_SECRET_KEY`. |
| `tls`        | optional | `false` | `true` = HTTPS. |

`validate` does **not** require any `minio.*` field — it never contacts the
server.

### `buckets[]`

| Field            | Required | Default | Notes |
|------------------|----------|---------|-------|
| `name`           | **yes**  | — | 3–63 chars, lowercase letters/digits/`.`/`-`, must start and end alphanumeric. Unique. |
| `description`    | optional | — | Free text, documentation only. Ignored by MinIO. |
| `region`         | optional | `us-east-1` | Set at creation; immutable afterwards. |
| `versioning`     | optional | `false` | Reconciled on every apply. |
| `object_locking` | optional | `false` | Only applied at bucket creation. |
| `quota_gb`       | optional | `0` | `0` = no quota. Applied at creation. |
| `lifecycle_days` | optional | `0` | `0` = no expiration rule. Applied at creation. |
| `tags`           | optional | — | `key: value` map. Reconciled on every apply. |

### `users[]`

| Field         | Required | Default | Notes |
|---------------|----------|---------|-------|
| `name`        | **yes**  | — | Access key. Unique. |
| `description` | optional | — | Free text, documentation only. |
| `password`    | **yes**  | — | Secret key. Plain value **or** a `vault://path#field` reference (see below). |
| `policies`    | optional | — | List of policy names. Each must be defined under `policies:` **or** be a MinIO built-in (`readwrite`, `readonly`, `writeonly`, `diagnostics`, `consoleAdmin`). |
| `enabled`     | optional | `false` | ⚠️ Defaults to `false` (disabled). Set `enabled: true` explicitly to enable the user. |

### `policies[]`

| Field         | Required | Default | Notes |
|---------------|----------|---------|-------|
| `name`        | **yes**  | — | Unique policy name. |
| `description` | optional | — | Free text, documentation only. |
| `statements`  | **yes**  | — | At least one statement. |

Each entry of `statements[]`:

| Field       | Required | Notes |
|-------------|----------|-------|
| `effect`    | **yes**  | `Allow` or `Deny`. |
| `actions`   | **yes**  | At least one S3/admin action, e.g. `s3:GetObject`. |
| `resources` | **yes**  | At least one ARN, e.g. `arn:aws:s3:::my-bucket/*`. |

> `description` is metadata only — it is stored in the YAML for humans, never
> sent to MinIO, and does not affect the diff.

## Configuration sources

Exactly one source *kind* must be selected: local file(s), Consul, or Vault.
Priority for connection values is always **CLI flag > ENV > config file**.

### From a local file

```bash
minio-manager apply --config configs/example.yaml \
  --minio-endpoint minio:9000 \
  --minio-access-key admin --minio-secret-key "$MINIO_SECRET_KEY"
```

### Multiple clusters in one run

`--config` is repeatable, and `--config-dir` picks up every `*.yaml`/`*.yml`
file in a directory. **Each file is a separate MinIO cluster** — it carries its
own `minio:` block and its own buckets/users/policies. Files are *not* merged;
`validate`/`plan`/`apply` process each target independently in order, print a
`=== <file> ===` header per target, and continue past a failing cluster,
returning a non-zero exit if any target failed.

```bash
# Explicit files, applied in the given order:
minio-manager apply --config dev.yaml --config qa.yaml --config prod.yaml

# Whole directory (files applied in alphabetical order):
minio-manager apply --config-dir ./clusters
```

Ordering: explicit `--config` files first (in the order given), then
`--config-dir` files sorted alphabetically; duplicates are applied once.

Because each cluster needs its own endpoint and credentials, the
`--minio-endpoint` / `--minio-access-key` / `--minio-secret-key` overrides are
**rejected when more than one target is present** — put those values in each
config's `minio:` block instead. (With a single target the overrides still
apply as usual.) Consul and Vault sources always resolve to exactly one target.

### From Consul KV

The YAML document is stored as the value of a single Consul key.

```bash
# 1. Publish the config to Consul
consul kv put minio/config @configs/example.yaml

# 2. Apply, reading the config from Consul
export CONSUL_HTTP_ADDR=127.0.0.1:8500
export CONSUL_HTTP_TOKEN=<acl-token>          # optional, if ACLs are enabled
minio-manager apply \
  --consul-key minio/config \
  --minio-endpoint minio:9000 \
  --minio-access-key admin --minio-secret-key "$MINIO_SECRET_KEY"
```

`--consul-addr` (or `CONSUL_HTTP_ADDR`) sets the agent address; `--consul-key`
is the KV key holding the YAML; the token is read from `CONSUL_HTTP_TOKEN`.

### From HashiCorp Vault (KV v2)

The YAML document is stored as a string field inside a KV v2 secret.

```bash
export VAULT_ADDR=https://vault:8200
export VAULT_TOKEN=<token>
minio-manager apply \
  --vault-path secret/data/minio/config \
  --vault-field config \
  --minio-endpoint minio:9000 \
  --minio-access-key admin --minio-secret-key "$MINIO_SECRET_KEY"
```

`--vault-field` defaults to `config`. The `data/` segment in the path is
optional — both `secret/data/minio/config` and `secret/minio/config` work.

### `vault://` password references

A user's `password` may be a reference instead of a literal value:

```yaml
users:
  - name: alice
    password: "vault://secret/minio/alice#password"
```

When `--vault-addr` (or `VAULT_ADDR`) is set, these references are resolved in
parallel at load time: the `password` field is read from the KV v2 secret at
`secret/minio/alice`. References work regardless of where the config itself
comes from (file, Consul or Vault).

## Environment variables

| Variable               | Equivalent flag        | Purpose |
|------------------------|------------------------|---------|
| `MINIO_MANAGER_CONFIG` | `--config`             | Path to a YAML config file (each file = one cluster). |
| `MINIO_MANAGER_CONFIG_DIR` | `--config-dir`     | Directory of `*.yaml`/`*.yml` config files (each = one cluster). |
| `LOG_LEVEL`            | `--log-level`          | `debug` \| `info` \| `warn` \| `error` (default `info`). |
| `MINIO_ENDPOINT`       | `--minio-endpoint`     | MinIO `host:port` (overrides `minio.endpoint`). |
| `MINIO_ACCESS_KEY`     | `--minio-access-key`   | MinIO access key (overrides `minio.access_key`). |
| `MINIO_SECRET_KEY`     | `--minio-secret-key`   | MinIO secret key (overrides `minio.secret_key`). |
| `CONSUL_HTTP_ADDR`     | `--consul-addr`        | Consul agent address. |
| `CONSUL_HTTP_TOKEN`    | *(env only)*           | Consul ACL token. |
| `VAULT_ADDR`           | `--vault-addr`         | Vault address. |
| `VAULT_TOKEN`          | *(env only)*           | Vault token. |

Flags without an ENV binding: `--log-format` (`text`\|`json`), `--consul-key`,
`--vault-path`, `--vault-field`, and `--prune` (on `plan`/`apply`).

`--log-format=json` emits machine-readable logs (to stderr) for CI; command
output such as the plan goes to stdout.

## Importing an existing cluster

Bootstrap a config from a running MinIO instance:

```bash
minio-manager import minio.example.com:9000 \
  --user admin --password "$MINIO_SECRET_KEY" \
  -o cluster.yaml
```

The output follows the same schema. Note that:

- user **passwords cannot be exported** (MinIO never returns secret keys) — fill
  them in or replace with `vault://` references before applying;
- `quota_gb` / `lifecycle_days` / `object_locking` are not exported yet (default
  to `0`/`false`);
- built-in policies are excluded unless `--include-builtin-policies` is set;
- the file is written with mode `0600` — **encrypt it (e.g. sops/age) before
  committing to git.**

## How apply works

1. Fetch current buckets, users and policies from MinIO (in parallel).
2. Compute the diff against the desired config.
3. Apply in phases: **policies** first (users depend on them), then **buckets +
   users** in parallel.
4. With `--prune`, delete resources absent from the config, in dependency-safe
   order (users → policies → buckets).

Deletions happen **only** with `--prune`. Passwords are never logged.
