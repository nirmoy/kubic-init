#
# Author(s): Alvaro Saurin <alvaro.saurin@suse.com>
#
# Copyright (c) 2017 SUSE LINUX GmbH, Nuernberg, Germany.
#
# All modifications and additions to the file contributed by third parties
# remain the property of their copyright owners, unless otherwise agreed
# upon. The license for this file, and modifications and additions to the
# file, is the same license as for the pristine package itself (unless the
# license for the pristine package is not an Open Source License, in which
# case the license is the MIT License). An "Open Source License" is a
# license that conforms to the Open Source Definition (Version 1.9)
# published by the Open Source Initiative.
#

variable "default_node_memory" {
  default     = 2048
  description = "default amount of RAM of the Nodes (in bytes)"
}

variable "img_pool" {
  default     = "default"
  description = "pool to be used to store all the volumes"
}

variable "img_url_base" {
  type        = "string"
  default     = "https://download.opensuse.org/repositories/devel:/kubic:/images:/experimental/images/"
  description = "URL to the KVM image used"
}

variable "img_src_filename" {
  type        = "string"
  default     = ""
  description = "Force a specific filename"
}

variable "img" {
  type        = "string"
  default     = "images/kubic.qcow2"
  description = "remote URL or local copy (can be used in conjuction with img_url_base) of the image to use."
}

variable "img_refresh" {
  default     = "true"
  description = "Try to get the latest image (true/false)"
}

variable "img_down_extra_args" {
  default     = ""
  description = "Extra arguments for the images downloader"
}

variable "img_sudo_virsh" {
  default     = "local"
  description = "Run virsh wioth sudo on [local|remote|both]"
}

variable "img_regex" {
  default = "MicroOS-docker-kvm-and-xen"
  description = "A string required in the image file name"
}

variable "kubic_init_image_name" {
  type        = "string"
  default     = "localhost/kubic-project/kubic-init:latest"
  description = "the default kubic init image name"
}

variable "kubic_init_image" {
  type        = "string"
  default     = "kubic-init-latest.tar.gz"
  description = "a kubic-init container image"
}

variable "libvirt_uri" {
  default     = "qemu:///system"
  description = "libvirt connection url (default to localhost)"
}

variable "nodes_count" {
  default     = 1
  description = "number of non-seed nodes to be created"
}

variable "network" {
  type        = "string"
  default     = "default"
  description = "an existing network to use for the VMs"
}

variable "nodes_memory" {
  default = {
    "3" = "1024"
    "4" = "1024"
    "5" = "1024"
  }

  description = "amount of RAM for some specific nodes"
}

variable "password" {
  type        = "string"
  default     = "linux"
  description = "password for sshing to the VMs"
}

variable "prefix" {
  type        = "string"
  default     = "kubic"
  description = "a prefix for resources"
}

variable "provision_script" {
  type        = "string"
  default     = "provision/devel.sh.tpl"
  description = "script used for provisioning the VM"
}

variable "pool" {
  type        = "string"
  default     = "default"
  description = "the libvirt pool"
}

variable "seeder" {
  type        = "string"
  default     = ""
  description = "an external seeder"
}

variable "token" {
  default = ""
  description = "kubeadm token"
}
