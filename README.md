# README.md

A simple tool which prints the AST of a HCL file  (e.g. Terraform file) as well
as extracts the position of some interesting declarations like variabels and
resource declarations.

Primarily created for use by https://github.com/mauve/vscode-terraform

# IMPORTANT

Requires this PR https://github.com/hashicorp/hcl/pull/196 to be merged, that PR is included in binary releases here https://github.com/mauve/terraform-index/releases.
