data "passwork_item" "db" {
  id = "abc123def456"
}

output "db_login" {
  value = data.passwork_item.db.login
}

output "db_password" {
  value     = data.passwork_item.db.password
  sensitive = true
}
