//go:build tools

package main

import (
	// tfplugindocs generates Terraform Registry documentation from provider schemas and examples.
	_ "github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs"
)
