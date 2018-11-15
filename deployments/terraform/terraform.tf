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

#######################
# Externals           #
#######################

# provisioning script
data "template_file" "provision_script" {
  template = "${file(var.provision_script)}"

  vars {
    kubic_init_image = "${var.kubic_init_image}"
  }
}

###########################
# Common                  #
###########################

module "kubic-nodes" {
  source = "./modules/kubic-node"

  default_node_memory   = "${var.default_node_memory}"
  kubic_init_image      = "${var.kubic_init_image}"
  kubic_init_image_name = "${var.kubic_init_image_name}"
  img                   = "${var.img}"
  img_down_extra_args   = "${var.img_down_extra_args}"
  img_pool              = "${var.img_pool}"
  img_refresh           = "${var.img_refresh}"
  img_src_filename      = "${var.img_src_filename}"
  img_sudo_virsh        = "${var.img_sudo_virsh}"
  img_regex             = "${var.img_regex}"
  img_url_base          = "${var.img_url_base}"
  libvirt_uri           = "${var.libvirt_uri}"
  network               = "${var.network}"
  nodes_count           = "${var.nodes_count}"
  nodes_memory          = "${var.nodes_memory}"
  password              = "${var.password}"
  pool                  = "${var.pool}"
  prefix                = "${var.prefix}"
  provision_script      = "${data.template_file.provision_script.rendered}"
  seeder                = "${var.seeder}"
  token                 = "${var.token}"
}

###########################
# Output                  #
###########################

output "seeder" {
  value = "${module.kubic-nodes.seeder}"
}

output "nodes" {
  value = "${module.kubic-nodes.addresses}"
}

output "token" {
  value = "${module.kubic-nodes.token}"
}
