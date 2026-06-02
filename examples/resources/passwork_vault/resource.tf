resource "passwork_vault" "example" {
  name = "Production Secrets"
}

# Import existing vault:
# terraform import passwork_vault.example <vault-id>
