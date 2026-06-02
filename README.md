# Terraform Provider for Passwork

Manages [Passwork](https://passwork.pro) vaults, folders, and password items via Terraform.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.21 (to build from source)
- A Passwork instance ≥ 7.6.0

## Installation

```hcl
terraform {
  required_providers {
    passwork = {
      source  = "alexey-mylnikov/passwork"
      version = "~> 0.1"
    }
  }
}
```

Run `terraform init` to install the provider from the Terraform Registry.

## Authentication

The provider uses API token pairs. Generate them in Passwork under **Profile → Authentication & 2FA → API Tokens → Generate pair**.

### Session caching (recommended)

On the first successful run the provider saves the refreshed token pair to `.terraform/passwork_session` (encrypted). Subsequent runs load the cached session automatically — you only need to supply fresh tokens after both have expired (typically months).

The encryption key is auto-generated and stored in `.terraform/passwork_session.key`. Both files are inside `.terraform/` which is git-ignored by default.

```hcl
provider "passwork" {
  host          = "https://passwork.example.com"
  access_token  = var.passwork_access_token   # only needed until first cache write
  refresh_token = var.passwork_refresh_token  # only needed until first cache write
}
```

Store initial tokens in environment variables or a secrets manager:

```sh
export TF_VAR_passwork_access_token="<access-token>"
export TF_VAR_passwork_refresh_token="<refresh-token>"
terraform init && terraform apply
```

### With client-side encryption

```hcl
provider "passwork" {
  host            = "https://passwork.example.com"
  access_token    = var.passwork_access_token
  refresh_token   = var.passwork_refresh_token
  master_password = var.passwork_master_password
}
```

### Provider configuration reference

| Attribute               | Env variable                    | Required    | Description |
|-------------------------|---------------------------------|-------------|-------------|
| `host`                  | `PASSWORK_HOST`                 | Yes         | Passwork instance URL |
| `access_token`          | `PASSWORK_ACCESS_TOKEN`         | First run   | API access token |
| `refresh_token`         | `PASSWORK_REFRESH_TOKEN`        | First run   | API refresh token |
| `master_password`       | `PASSWORK_MASTER_PASSWORD`      | No          | Master password for client-side encryption |
| `master_key`            | `PASSWORK_MASTER_KEY`           | No          | Pre-derived master key (alternative to `master_password`) |
| `skip_tls_verify`       | —                               | No          | Disable TLS verification (dev only) |
| `session_cache_file`    | `PASSWORK_SESSION_CACHE_FILE`   | No          | Session cache path (default: `.terraform/passwork_session`) |
| `session_encryption_key`| `PASSWORK_SESSION_ENCRYPTION_KEY` | No        | Explicit hex AES key for session file encryption |

## Resources

### `passwork_vault`

```hcl
resource "passwork_vault" "example" {
  name    = "Production Secrets"
  type_id = "optional-vault-type-id"  # required if vaultType feature is enabled
}
```

**Arguments:** `name` (required), `type_id` (optional)  
**Attributes:** `id`

**Import:**
```sh
terraform import passwork_vault.example <vault-id>
```

---

### `passwork_folder`

```hcl
resource "passwork_folder" "example" {
  name             = "Databases"
  vault_id         = passwork_vault.example.id
  parent_folder_id = passwork_folder.parent.id  # optional
  color            = 2                           # optional
}
```

**Arguments:** `name` (required), `vault_id` (required, forces replacement), `parent_folder_id` (optional), `color` (optional)  
**Attributes:** `id`

**Import:**
```sh
terraform import passwork_folder.example <folder-id>
```

---

### `passwork_item`

```hcl
resource "passwork_item" "example" {
  name        = "PostgreSQL prod"
  vault_id    = passwork_vault.example.id
  folder_id   = passwork_folder.example.id  # optional
  login       = "postgres"
  password    = var.db_password             # sensitive
  url         = "postgresql://prod.db.example.com:5432/mydb"
  description = "Production database credentials"
  tags        = ["database", "production"]
  color_code  = 1

  custom_fields = [
    { name = "Port", type = "text",     value = "5432"    },
    { name = "SSL",  type = "password", value = "require" },
  ]
}
```

**Arguments:** `name`, `vault_id` (required, forces replacement), `folder_id`, `login`, `password` (sensitive), `url`, `description`, `tags`, `color_code`, `custom_fields`  
**Attributes:** `id`

> On `terraform destroy` the item is moved to the Passwork bin (soft delete), not permanently removed.

**Import:**
```sh
terraform import passwork_item.example <item-id>
```

## Data Sources

### `data.passwork_vault`

Looks up a vault by name.

```hcl
data "passwork_vault" "prod" {
  name = "Production Secrets"
}
```

### `data.passwork_folder`

Looks up a folder by name within a specific vault.

```hcl
data "passwork_folder" "infra" {
  name     = "Infrastructure"
  vault_id = data.passwork_vault.prod.id
}
```

### `data.passwork_item`

Fetches a password item by ID (including decrypted password when encryption is configured).

```hcl
data "passwork_item" "db" {
  id = "abc123def456"
}

output "db_password" {
  value     = data.passwork_item.db.password
  sensitive = true
}
```

## Token refresh and rotation

Passwork 7.6.0+ uses short-lived access tokens and long-lived refresh tokens. When `access_token` expires mid-run, the provider transparently calls `POST /api/v1/sessions/refresh`, which issues a **new pair** and invalidates the old one. The new pair is persisted to the session cache immediately, so the next Terraform run picks it up automatically.

## Publishing a release

1. Tag the commit: `git tag v0.1.0 && git push origin v0.1.0`
2. GitHub Actions runs GoReleaser, builds multi-platform binaries, signs checksums with your GPG key, and creates a GitHub Release.
3. The Terraform Registry picks up the release automatically via webhook (configure it once at [registry.terraform.io](https://registry.terraform.io)).

**Required GitHub Secrets:**

| Secret            | Description |
|-------------------|-------------|
| `GPG_PRIVATE_KEY` | ASCII-armored GPG private key registered in Terraform Registry |
| `GPG_PASSPHRASE`  | Passphrase for the GPG key |

## Development

```sh
# Build
make build

# Run tests
make test

# Format + vet
make lint

# Install locally (uses ~/.terraform.d/plugins)
make install
```

## License

[MIT](LICENSE)
