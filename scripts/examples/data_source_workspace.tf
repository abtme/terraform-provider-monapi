data "monapi_workspace" "this" {}

output "workspace" {
  value = "${data.monapi_workspace.this.name} (token role: ${data.monapi_workspace.this.role})"
}
