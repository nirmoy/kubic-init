# NOTE: this only works when installed in a GOPATH dir
GOPATH_THIS_USER = $(shell basename `realpath ..`)
GOPATH_THIS_REPO = $(shell basename `pwd`)

# go source files, ignore vendor directory
KUBIC_INIT_EXE  = cmd/kubic-init/kubic-init
KUBIC_INIT_SH   = build/image/entrypoint.sh
KUBIC_INIT_MAIN = cmd/kubic-init/main.go
KUBIC_INIT_SRCS = $(shell find . -type f -name '*.go' -not -path "./vendor/*")
KUBIC_INIT_CFGS = $(shell find ./configs -type f -not -name "README*")
.DEFAULT_GOAL: $(KUBIC_INIT_EXE)

IMAGE_BASENAME = $(GOPATH_THIS_REPO)
IMAGE_NAME     = $(GOPATH_THIS_USER)/$(IMAGE_BASENAME)
IMAGE_TAR_GZ   = $(IMAGE_BASENAME)-latest.tar.gz
IMAGE_DEPS     = $(KUBIC_INIT_EXE) $(KUBIC_INIT_SH) $(KUBIC_INIT_CFGS) Dockerfile

# should be non-empty when dep is installed
DEP_EXE := $(shell command -v dep 2> /dev/null)

# These will be provided to the target
KUBIC_INIT_VERSION := 1.0.0
KUBIC_INIT_BUILD   := `git rev-parse HEAD 2>/dev/null`

# Use linker flags to provide version/build settings to the target
KUBIC_INIT_LDFLAGS = -ldflags "-X=main.Version=$(KUBIC_INIT_VERSION) -X=main.Build=$(KUBIC_INIT_BUILD)"

MANIFEST_LOCAL = deployments/kubelet/kubic-init-manifest.yaml
MANIFEST_REM   = deployments/kubic-init-manifest.yaml
MANIFEST_DIR   = /etc/kubernetes/manifests

KUBE_DROPIN_SRC = init/kubelet.drop-in.conf
KUBE_DROPIN_DST = /etc/systemd/system/kubelet.service.d/kubelet.drop-in.conf

TF_LIBVIRT_FULL_DIR  = deployments/tf-libvirt-full
TF_LIBVIRT_NODES_DIR = deployments/tf-libvirt-nodes
TF_ARGS_DEFAULT      = -var 'kubic_init_image=$(IMAGE_TAR_GZ)'

# sudo command (and version passing env vars)
SUDO = sudo
SUDO_E = $(SUDO) -E

# increase to 8 for detailed kubeadm logs...
# Example: make local-run VERBOSE_LEVEL=8
VERBOSE_LEVEL = 5

CONTAINER_VOLUMES = \
		-v `pwd`/configs:/etc/kubic \
        -v /etc/kubernetes:/etc/kubernetes \
        -v /etc/hosts:/etc/hosts:ro \
        -v /usr/bin/kubelet:/usr/bin/kubelet:ro \
        -v /var/lib/kubelet:/var/lib/kubelet \
        -v /etc/cni/net.d:/etc/cni/net.d \
        -v /var/lib/dockershim:/var/lib/dockershim \
        -v /var/lib/etcd:/var/lib/etcd \
        -v /sys/fs/cgroup:/sys/fs/cgroup \
        -v /var/run:/var/run

#############################################################
# Build targets
#############################################################

all: $(KUBIC_INIT_EXE)

dep-exe:
ifndef DEP_EXE
	@echo ">>> dep does not seem to be installed. installing dep..."
	go get -u github.com/golang/dep/cmd/dep
endif

dep-rebuild: dep-exe Gopkg.toml
	@echo ">>> Rebuilding vendored deps (respecting Gopkg.toml constraints)"
	rm -rf vendor Gopkg.lock
	dep ensure -v && dep status

dep-update: dep-exe Gopkg.toml
	@echo ">>> Updating vendored deps (respecting Gopkg.toml constraints)"
	dep ensure -update -v && dep status

# download automatically the vendored deps when "vendor" doesn't exist
vendor: dep-exe
	@[ -d vendor ] || dep ensure -v

$(KUBIC_INIT_EXE): $(KUBIC_INIT_SRCS) Gopkg.lock vendor
	@echo ">>> Building $(KUBIC_INIT_EXE)..."
	go build $(KUBIC_INIT_LDFLAGS) -o $(KUBIC_INIT_EXE) $(KUBIC_INIT_MAIN)

.PHONY: fmt
fmt:
	@gofmt -l -w $(KUBIC_INIT_SRCS)

.PHONY: simplify
simplify:
	@gofmt -s -l -w $(KUBIC_INIT_SRCS)

.PHONY: check
check:
	@test -z $(shell gofmt -l $(KUBIC_INIT_MAIN) | tee /dev/stderr) || echo "[WARN] Fix formatting issues with 'make fmt'"
	@for d in $$(go list ./... | grep -v /vendor/); do golint $${d}; done
	@go tool vet ${KUBIC_INIT_SRCS}

.PHONY: check
clean: docker-reset kubelet-reset docker-image-clean
	rm -f $(KUBIC_INIT_EXE)

#############################################################
# Some simple run targets
# (for testing things locally)
#############################################################

# we must "patch" the local kubelet by adding a drop-in unit
# otherwise, the kubelet will be run with the wrong arguments
/var/lib/kubelet/config.yaml: /etc/kubernetes/kubelet-config.yaml
	$(SUDO) cp -f /etc/kubernetes/kubelet-config.yaml /var/lib/kubelet/config.yaml

$(KUBE_DROPIN_DST): $(KUBE_DROPIN_SRC) /var/lib/kubelet/config.yaml
	@echo ">>> Adding drop-in unit for the local kubelet"
	$(SUDO) mkdir -p `dirname $(KUBE_DROPIN_DST)`
	$(SUDO) cp -f $(KUBE_DROPIN_SRC) $(KUBE_DROPIN_DST)
	$(SUDO) systemctl daemon-reload

kubeadm-reset: local-reset
local-reset: $(KUBIC_INIT_EXE)
	@echo ">>> Resetting everything..."
	$(SUDO_E) $(KUBIC_INIT_EXE) reset -v $(VERBOSE_LEVEL) -f


# Usage:
#  - create a local seeder: make local-run
#  - create a local seeder with a specific token: TOKEN=XXXX make local-run
#  - join an existing seeder: env SEEDER=1.2.3.4 TOKEN=XXXX make local-run
local-run: $(KUBIC_INIT_EXE) $(KUBE_DROPIN_DST) local-reset
	@echo ">>> Running $(KUBIC_INIT_EXE) as _root_"
	$(SUDO_E) $(KUBIC_INIT_EXE) bootstrap \
		-v $(VERBOSE_LEVEL) \
		--config configs/kubic-init.yaml

# Usage:
#  - create a local seeder: make docker-run
#  - create a local seeder with a specific token: TOKEN=XXXX make docker-run
#  - join an existing seeder: env SEEDER=1.2.3.4 TOKEN=XXXX make docker-run
docker-run: $(IMAGE_TAR_GZ) docker-reset $(KUBE_DROPIN_DST)
	@echo ">>> Running $(IMAGE_NAME):latest in the local Docker"
	docker run -it --rm \
		--privileged=true \
		--net=host \
		--security-opt seccomp:unconfined \
		--cap-add=SYS_ADMIN \
		--name=$(IMAGE_BASENAME) \
		-e SEEDER \
		-e TOKEN \
		$(CONTAINER_VOLUMES) \
		$(IMAGE_NAME):latest

docker-reset: kubeadm-reset

$(IMAGE_TAR_GZ): $(IMAGE_DEPS)
	@echo ">>> Creating Docker image..."
	docker build \
		--build-arg KUBIC_INIT_EXE=$(KUBIC_INIT_EXE) \
		--build-arg KUBIC_INIT_SH=$(KUBIC_INIT_SH) \
		-t $(IMAGE_NAME):latest .
	@echo ">>> Creating tar for image..."
	docker save $(IMAGE_NAME):latest | gzip > $(IMAGE_TAR_GZ)

docker-image: $(IMAGE_TAR_GZ)
docker-image-clean:
	rm -f $(IMAGE_TAR_GZ)
	-docker rmi $(IMAGE_NAME)

# TODO: build the image for podman
# TODO: implement podman-reset
podman-run: podman-image podman-reset $(KUBE_DROPIN_DST)
	$(SUDO_E) podman run -it --rm \
		--privileged=true \
		--net=host \
		--security-opt seccomp:unconfined \
		--cap-add=SYS_ADMIN \
		--name=$(IMAGE_BASENAME) \
		-h master \
		-e SEEDER \
		-e TOKEN \
		$(CONTAINER_VOLUMES) \
		$(IMAGE_NAME):latest

kubelet-run: $(IMAGE_TAR_GZ) kubelet-reset $(KUBE_DROPIN_DST)
	@echo ">>> Pushing $(IMAGE_NAME):latest to docker Hub"
	docker push $(IMAGE_NAME):latest
	@echo ">>> Copying manifest to $(MANIFEST_DIR) (will require root password)"
	mkdir -p $(MANIFEST_DIR)
	$(SUDO) cp -f $(MANIFEST_LOCAL) $(MANIFEST_DIR)/`basename $(MANIFEST_REM)`
	$(SUDO) systemctl restart kubelet
	@echo ">>> Manifest copied. Waiting for kubelet to start things..."
	@while ! docker ps | grep $(IMAGE_BASENAME) | grep -q -v pause ; do echo "Waiting container..." ; sleep 2 ; done
	@docker logs -f "`docker ps | grep $(IMAGE_BASENAME) | grep -v pause | cut -d' ' -f1`"

kubelet-reset: kubeadm-reset
	@echo ">>> Resetting everything..."
	@echo ">>> Stopping the kubelet..."
	@$(SUDO) systemctl stop kubelet
	@while [ ! -e /var/run/docker.sock   ] ; do echo "Waiting for dockers socket..."     ; sleep 2 ; done
	@while [ -e /var/run/dockershim.sock ] ; do echo "Waiting until the kubelet is down..." ; sleep 2 ; done
	@echo ">>> Restoring a safe kubelet configuration..."
	$(SUDO) cp /etc/kubernetes/kubelet-config.yaml /var/lib/kubelet/config.yaml
	@-rm -f $(MANIFEST_DIR)/$(MANIFEST_REM)


#############################################################
# Terraform deployments
#############################################################

### Terraform full deplyment

$(TF_LIBVIRT_FULL_DIR)/.terraform:
	cd $(TF_LIBVIRT_FULL_DIR) && terraform init

tf-full-plan: $(TF_LIBVIRT_FULL_DIR)/.terraform
	cd $(TF_LIBVIRT_FULL_DIR) && terraform plan

# Usage:
# - create a only-one-seeder cluster: make tf-full-run TF_ARGS="-var nodes_count=0"
tf-full-run: tf-full-apply
tf-full-apply: $(TF_LIBVIRT_FULL_DIR)/.terraform $(IMAGE_TAR_GZ)
	@echo ">>> Deploying a full cluster with Terraform..."
	cd $(TF_LIBVIRT_FULL_DIR) && terraform apply $(TF_ARGS_DEFAULT) $(TF_ARGS)

tf-full-reapply:
	cd $(TF_LIBVIRT_FULL_DIR) && terraform apply $(TF_ARGS_DEFAULT) $(TF_ARGS)

tf-full-destroy: $(TF_LIBVIRT_FULL_DIR)/.terraform
	cd $(TF_LIBVIRT_FULL_DIR) && terraform destroy -force $(TF_ARGS_DEFAULT) $(TF_ARGS)

tf-full-nuke:
	-make tf-full-destroy
	cd $(TF_LIBVIRT_FULL_DIR) && rm -f *.tfstate*

### Terraform only-nodes deployment

$(TF_LIBVIRT_NODES_DIR)/.terraform:
	cd $(TF_LIBVIRT_NODES_DIR) && terraform init

tf-nodes-plan: $(TF_LIBVIRT_NODES_DIR)/.terraform
	cd $(TF_LIBVIRT_NODES_DIR) && terraform plan

tf-nodes-run: tf-nodes-apply
tf-nodes-apply: $(TF_LIBVIRT_NODES_DIR)/.terraform $(IMAGE_TAR_GZ)
	@echo ">>> Deploying only-nodes with Terraform..."
	cd $(TF_LIBVIRT_NODES_DIR) && terraform apply $(TF_ARGS_DEFAULT) $(TF_ARGS)

tf-nodes-reapply:
	cd $(TF_LIBVIRT_NODES_DIR) && terraform apply $(TF_ARGS_DEFAULT) $(TF_ARGS)

tf-nodes-destroy: $(TF_LIBVIRT_NODES_DIR)/.terraform
	cd $(TF_LIBVIRT_NODES_DIR) && terraform destroy -force $(TF_ARGS_DEFAULT) $(TF_ARGS)

tf-nodes-nuke:
	-make tf-nodes-destroy
	cd $(TF_LIBVIRT_NODES_DIR) && rm -f *.tfstate*

#############################################################
# Other stuff
#############################################################



