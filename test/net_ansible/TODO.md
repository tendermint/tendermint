 1. Dynamically get SSH key ID to start Digital Ocean droplets through Terraform as output of the following command
# curl -X GET -H "Content-Type: application/json" -H "Authorization: Bearer $DIGITALOCEAN_TOKEN" "https://api.digitalocean.com/v2/account/keys"

 2. Generate a dynamic Ansible inventory from a Terraform state file. [https://github.com/adammck/terraform-inventory]

 3. Optionally: use S3 to store Terraform inventory

 4. Modify step "Copy priv_validator.json" to use any given amount of "machN" folders

 5. detect what operating system is running locally and copy appropriate tendermint binary

 6. Use Ubuntu 16.04 instead of 14.04
