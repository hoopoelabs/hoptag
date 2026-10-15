# Google compute engine instance
resource "google_compute_instance" "app" {
  name         = "app-server"
  machine_type = "e2-medium"
  zone         = "us-central1-a"
}

# Google Cloud Storage bucket
resource "google_storage_bucket" "static" {
  name     = "app-static-content-bucket"
  location = "US"
}
