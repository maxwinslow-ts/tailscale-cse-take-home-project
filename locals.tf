# Shared network constants used by ACL policy and auto-approvers.
locals {
  us_docker_cidr = "172.21.0.0/24" # Docker bridge on us-app; used in ACL grants & route approvals
}