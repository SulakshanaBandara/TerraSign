resource "null_resource" "valid" {
  triggers = {
    build_number = "${timestamp()}"
  }
}
