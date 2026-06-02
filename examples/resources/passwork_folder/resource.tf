resource "passwork_vault" "main" {
  name = "My Vault"
}

resource "passwork_folder" "root" {
  name     = "Infrastructure"
  vault_id = passwork_vault.main.id
}

resource "passwork_folder" "nested" {
  name             = "Databases"
  vault_id         = passwork_vault.main.id
  parent_folder_id = passwork_folder.root.id
}

# Import existing folder:
# terraform import passwork_folder.root <folder-id>
