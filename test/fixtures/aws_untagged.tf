# Production web server
resource "aws_instance" "web" {
  ami           = "ami-0c55b159cbfafe1f0"
  instance_type = "t3.micro"
}

# S3 bucket for assets
resource "aws_s3_bucket" "assets" {
  bucket = "my-company-assets-prod"
}
