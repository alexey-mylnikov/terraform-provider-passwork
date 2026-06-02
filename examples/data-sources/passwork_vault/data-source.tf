data "passwork_vault" "prod" {
  name = "Production Secrets"
}

output "vault_id" {
  value = data.passwork_vault.prod.id
}
