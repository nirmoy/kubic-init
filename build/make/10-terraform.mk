TF_LIBVIRT_FULL_DIR  = deployments/tf-libvirt-full
TF_LIBVIRT_NODES_DIR = deployments/tf-libvirt-nodes
TF_ARGS_DEFAULT      = -input=false -auto-approve \
					   -var 'kubic_init_image_tgz="$(IMAGE_TAR_GZ)"' \
					   -var 'kubic_init_extra_args="$(KUBIC_INIT_EXTRA_ARGS)"'

SSH_ARGS := -o "StrictHostKeyChecking=no"
SSH_VMS  := $(shell command sshpass >/dev/null 2>&1 && echo "sshpass -p linux ssh $(SSH_ARGS)" || echo "ssh $(SSH_ARGS)")

#############################################################
# Terraform deployments
#############################################################

### Terraform full deplyment

tf-full-plan:
	cd $(TF_LIBVIRT_FULL_DIR) && terraform init && terraform plan

#
# Usage:
# - create a only-one-seeder cluster:
#   $ make tf-full-run TF_ARGS="-var nodes_count=0"
# - create cluster with Docker:
#   $ make tf-full-run TF_ARGS="-var kubic_init_runner=docker" \
#    KUBIC_INIT_EXTRA_ARGS="--var Runtime.Engine=docker"
#
tf-full-run: tf-full-apply
tf-full-apply: docker-image
	@echo ">>> Deploying a full cluster with Terraform..."
	cd $(TF_LIBVIRT_FULL_DIR) && terraform init && terraform apply $(TF_ARGS_DEFAULT) $(TF_ARGS)

tf-full-reapply:
	cd $(TF_LIBVIRT_FULL_DIR) && terraform init && terraform apply $(TF_ARGS_DEFAULT) $(TF_ARGS)

tf-full-destroy:
	cd $(TF_LIBVIRT_FULL_DIR) && terraform init && terraform destroy -force $(TF_ARGS_DEFAULT) $(TF_ARGS)

tf-full-nuke:
	-make tf-full-destroy
	cd $(TF_LIBVIRT_FULL_DIR) && rm -f *.tfstate*

### Terraform only-seeder deployment (shortcut for `nodes_count=0`)

tf-seeder-plan:
	-make tf-full-plan TF_ARGS="-var nodes_count=0 $(TF_ARGS)"

#
# Usage:
# - create a seeder with a specific Token:
#   $ env TOKEN=XXXX make tf-seeder-run
#
tf-seeder-run: tf-seeder-apply
tf-seeder-apply:
	@echo ">>> Deploying only-seeder with Terraform..."
	@make tf-full-apply TF_ARGS="-var nodes_count=0 $(TF_ARGS)"

tf-seeder-reapply:
	@make tf-full-reapply TF_ARGS="-var nodes_count=0 $(TF_ARGS)"

tf-seeder-destroy:
	@make tf-full-destroy TF_ARGS="-var nodes_count=0 $(TF_ARGS)"

tf-seeder-nuke: tf-full-nuke

### Terraform only-nodes deployment

tf-nodes-plan:
	cd $(TF_LIBVIRT_NODES_DIR) && terraform init && terraform plan

#
# Usage:
# - create only one node (ie, for connecting to the seeder started locally with `make local-run`):
#   $ env TOKEN=XXXX make tf-nodes-run
#
tf-nodes-run: tf-nodes-apply
tf-nodes-apply: docker-image
	@echo ">>> Deploying only-nodes with Terraform..."
	cd $(TF_LIBVIRT_NODES_DIR) && terraform init && terraform apply $(TF_ARGS_DEFAULT) $(TF_ARGS)

tf-nodes-destroy: $(TF_LIBVIRT_NODES_DIR)/.terraform
	cd $(TF_LIBVIRT_NODES_DIR) && terraform init && terraform destroy -force $(TF_ARGS_DEFAULT) $(TF_ARGS)

tf-nodes-nuke:
	-make tf-nodes-destroy
	cd $(TF_LIBVIRT_NODES_DIR) && rm -f *.tfstate*

#
# some ssh convenience targets
#
SEEDER := $(shell cd $(TF_LIBVIRT_FULL_DIR) && terraform output -json seeder 2>/dev/null | python -c 'import sys, json; print json.load(sys.stdin)["value"]' 2>/dev/null)
tf-ssh-seeder:
	$(SSH_VMS) root@$(SEEDER)

NODE0 := $(shell cd $(TF_LIBVIRT_FULL_DIR) && terraform output -json nodes 2>/dev/null | python -c 'import sys, json; print json.load(sys.stdin)["value"][0][0]' 2>/dev/null)
tf-ssh-node-0:
	@$(SSH_VMS) root@$(NODE0)
