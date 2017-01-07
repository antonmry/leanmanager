variable "disk_name" {
  default = "leanmanager-disk"
}

resource "google_compute_disk" "default" {
name  = "${var.disk_name}"
  type  = "pd-ssd"
  zone = "${var.region}"
  size  = "1"
}
