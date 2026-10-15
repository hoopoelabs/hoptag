resource "aws_instance" "dynamic_app" {
  ami           = "ami-88888"
  instance_type = "c5.xlarge"
  tags          = var.common_tags
}
