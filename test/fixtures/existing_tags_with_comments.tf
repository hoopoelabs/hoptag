# Server with existing tags and comments
resource "aws_instance" "db" {
  ami           = "ami-99999"
  instance_type = "m5.large"

  # Important database tags
  tags = {
    # Keep this custom tag!
    BackupPlan  = "Daily"
    Environment = "dev" # inline comment
  }
}
