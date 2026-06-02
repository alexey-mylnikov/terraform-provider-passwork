data "passwork_vault" "prod" {
  name = "Production Secrets"
}

data "passwork_folder" "infra" {
  name     = "Infrastructure"
  vault_id = data.passwork_vault.prod.id
}

output "folder_id" {
  value = data.passwork_folder.infra.id
}
