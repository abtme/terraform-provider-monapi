terraform {
  required_providers {
    monapi = {
      source = "abtme/monapi"
    }
  }
}

# Or set MONAPI_URL and MONAPI_TOKEN.
provider "monapi" {
  url   = "https://monitor.example.com"
  token = var.monapi_token
}

variable "monapi_token" {
  type      = string
  sensitive = true
}
