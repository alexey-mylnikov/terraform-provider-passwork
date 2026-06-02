data "passwork_vault_type" "default" {
  code = "default"
}

resource "passwork_vault" "example" {
  name    = "My Vault"
  type_id = data.passwork_vault_type.default.id
}
