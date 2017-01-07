variable "region" {
  default = "europe-west1-d"
}

variable "cluster_name" {
  default = "leanmanager-cluster"
}

variable "LEANMANAGER_TOKEN" {
  default = "USE YOUR OWN TOKEN"
}

provider "google" {
  credentials = "${file("account.json")}"
  project     = "wwwleanmanagereu"
  region      = "${var.region}"
}

resource "google_container_cluster" "primary" {
  name = "${var.cluster_name}"
  zone = "${var.region}"
  initial_node_count = 1

  master_auth {
    username = "mr.yoda"
    password = "testTest1"
  }

  node_config {
    oauth_scopes = [
      "https://www.googleapis.com/auth/compute",
      "https://www.googleapis.com/auth/devstorage.read_only",
      "https://www.googleapis.com/auth/logging.write",
      "https://www.googleapis.com/auth/monitoring"
    ]
  }

  provisioner "local-exec" {
    command = "gcloud container clusters get-credentials ${var.cluster_name} --zone ${var.region}"
  }

provisioner "local-exec" {
    command = "cp leanmanager-pod-template.yaml leanmanager.tmp.yaml && sed -i -- 's/LEANMANAGER_TOKEN_TEMPLATE/${var.LEANMANAGER_TOKEN}/g' leanmanager.tmp.yaml"
  }

  provisioner "local-exec" {
    command = "kubectl create -f leanmanager.tmp.yaml"
  }

  provisioner "local-exec" {
    command = "rm -f leanmanager.tmp.yaml"
  }
}
