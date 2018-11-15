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

provider "libvirt" {
  uri = "${var.libvirt_uri}"
}

data "external" "get_seeder" {
  program = [
    "python",
    "${path.module}/get-seeder.py",
    "${var.seeder} ",
  ] # NOTE: the space is intentional (the string cannot be empty)
}

data "external" "get_token" {
  program = [
    "python",
    "${path.module}/get-token.py",
    "${var.token} ",
  ] # NOTE: the space is intentional (the string cannot be empty)
}

#################################################
# base image
#################################################

locals {
  "token"            = "${data.external.get_token.result.token}"
  "external_addr"    = "${data.external.get_seeder.result.address}"
  "create_seeder"    = "${data.external.get_seeder.result.existing == "true" ? false : true }"
  "total_node_count" = "${local.create_seeder ? var.nodes_count + 1 : var.nodes_count }"
  "create_nodes"     = "${local.total_node_count > 0}"
}

resource "null_resource" "download_kubic_image" {
  count = "${length(var.img_url_base) > 0 && (local.create_nodes || local.create_seeder) ? 1 : 0}"

  provisioner "local-exec" {
    command = "${path.module}/download-image.sh --sudo-virsh '${var.img_sudo_virsh}' --src-base '${var.img_url_base}' --regex '${var.img_regex}' --refresh '${var.img_refresh}' --local '${var.img}' --upload-to-img '${var.prefix}_base_${basename(var.img)}' --upload-to-pool '${var.img_pool}' --src-filename '${var.img_src_filename}' ${var.img_down_extra_args}"
  }
}

resource "libvirt_volume" "node" {
  count            = "${local.total_node_count}"
  name             = "${var.prefix}_node_${count.index}.qcow2"
  pool             = "${var.pool}"
  base_volume_name = "${var.prefix}_base_${basename(var.img)}"
  depends_on       = ["null_resource.download_kubic_image"]
}

#################################################
# cloud-init
#################################################

data "template_file" "node_cloud_init_user_data" {
  count    = "${local.total_node_count}"
  template = "${file("${path.module}/cloud-init/cloud-init.yaml")}"

  vars {
    password = "${var.password}"
    hostname = "${var.prefix}-node-${count.index}"
  }
}

resource "libvirt_cloudinit_disk" "node" {
  count     = "${local.total_node_count}"
  name      = "${var.prefix}_node_cloud_init_${count.index}.iso"
  pool      = "${var.pool}"
  user_data = "${element(data.template_file.node_cloud_init_user_data.*.rendered, count.index)}"
}

#################################################
# domains
#################################################

resource "libvirt_domain" "node" {
  count      = "${local.total_node_count}"
  name       = "${var.prefix}-node-${count.index}"
  memory     = "${lookup(var.nodes_memory, count.index, var.default_node_memory)}"
  cloudinit  = "${element(libvirt_cloudinit_disk.node.*.id, count.index)}"
  depends_on = ["libvirt_volume.node", "libvirt_cloudinit_disk.node"]

  cpu {
    mode = "host-passthrough"
  }

  disk {
    volume_id = "${element(libvirt_volume.node.*.id, count.index)}"
  }

  network_interface {
    network_name   = "${var.network}"
    wait_for_lease = 1
  }

  graphics {
    type        = "vnc"
    listen_type = "address"
  }
}

locals {
  "seeder_ip" = "${local.create_seeder ? element(concat(libvirt_domain.node.*.network_interface.0.addresses, list("")), 0) : local.external_addr }"
}

#################################################
# kubic-init.yaml
#################################################

data "template_file" "kubic_init_config" {
  count      = "${local.total_node_count}"
  template   = "${local.create_seeder && count.index == 0 ?  file("${path.module}/kubic-init/seeder.yaml"): file("${path.module}/kubic-init/node.yaml") }"
  depends_on = ["libvirt_domain.node"]

  vars {
    seeder   = "${local.create_seeder && count.index == 0 ?  "": local.seeder_ip}"
    token    = "${local.token}"
    password = "${var.password}"
    hostname = "${var.prefix}-node-${count.index}"
  }
}

resource "null_resource" "upload_kubic_init" {
  count      = "${local.total_node_count}"
  depends_on = ["libvirt_domain.node"]

  connection {
    host     = "${element(libvirt_domain.node.*.network_interface.0.addresses.0, count.index)}"
    password = "${var.password}"
  }

  provisioner "file" {
    content     = "${element(data.template_file.kubic_init_config.*.rendered, count.index)}"
    destination = "/etc/systemd/system/kubelet.service.d/kubelet.conf"
  }
}

#################################################
# provision
#################################################

resource "null_resource" "upload_config_nodes" {
  count      = "${length(var.provision_script) > 0 ? local.total_node_count : 0}"
  depends_on = ["libvirt_domain.node", "null_resource.upload_kubic_init"]

  connection {
    host     = "${element(libvirt_domain.node.*.network_interface.0.addresses.0, count.index)}"
    password = "${var.password}"
  }

  provisioner "remote-exec" {
    inline = [
      "mkdir -p /etc/systemd/system/kubelet.service.d",
    ]
  }

  provisioner "file" {
    source      = "${path.root}/../../init/kubelet.drop-in.conf"
    destination = "/etc/systemd/system/kubelet.service.d/kubelet.conf"
  }

  provisioner "file" {
    source      = "${path.root}/../../init/kubic-init.systemd.conf"
    destination = "/etc/systemd/system/kubic-init.service"
  }

  provisioner "file" {
    source      = "${path.root}/../../init/kubic-init.sysconfig"
    destination = "/etc/sysconfig/kubic-init"
  }

  provisioner "file" {
    source      = "${path.root}/../../init/kubelet-sysctl.conf"
    destination = "/etc/sysctl.d/99-kubernetes-cri.conf"
  }

  provisioner "file" {
    source      = "${path.root}/../../${var.kubic_init_image}"
    destination = "/tmp/${var.kubic_init_image}"
  }

  # TODO: this is only for development
  provisioner "remote-exec" {
    inline = "${var.provision_script}"
  }
}
