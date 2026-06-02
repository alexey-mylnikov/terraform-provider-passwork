terraform {
  required_providers {
    passwork = {
      source  = "alexey-mylnikov/passwork"
      version = "~> 0.1"
    }
  }
}

# Minimal configuration — tokens are only required on the first run.
# After that, refreshed tokens are cached in .terraform/passwork_session.
provider "passwork" {
  host          = "https://passwork.example.com"
  access_token  = var.passwork_access_token
  refresh_token = var.passwork_refresh_token
}

variable "passwork_access_token" {
  type      = string
  sensitive = true
}

variable "passwork_refresh_token" {
  type      = string
  sensitive = true
}
