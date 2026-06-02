resource "passwork_vault" "main" {
  name = "My Vault"
}

resource "passwork_folder" "infra" {
  name     = "Infrastructure"
  vault_id = passwork_vault.main.id
}

resource "passwork_item" "example" {
  name        = "PostgreSQL prod"
  vault_id    = passwork_vault.main.id
  folder_id   = passwork_folder.infra.id
  login       = "postgres"
  password    = var.db_password
  url         = "postgresql://prod.db.example.com:5432/mydb"
  description = "Production PostgreSQL credentials"
  tags        = ["database", "production"]
  color_code  = 1

  custom_fields = [
    {
      name  = "Port"
      type  = "text"
      value = "5432"
    },
    {
      name  = "SSL Mode"
      type  = "text"
      value = "require"
    },
  ]
}

variable "db_password" {
  type      = string
  sensitive = true
}

# Import existing item:
# terraform import passwork_item.example <item-id>
