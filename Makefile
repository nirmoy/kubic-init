# NOTE: this only works when installed in a GOPATH dir
GOPATH_THIS_USER = $(shell basename `realpath ..`)
GOPATH_THIS_REPO = $(shell basename `pwd`)

SOURCES_DIRS    = cmd pkg
SOURCES_DIRS_GO = ./pkg/... ./cmd/...

# go source files, ignore vendor directory
KUBIC_INIT_SRCS      = $(shell find $(SOURCES_DIRS) -type f -name '*.go' -not -path "*generated*")
KUBIC_INIT_MAIN_SRCS = $(shell find $(SOURCES_DIRS) -type f -name '*.go' -not -path "*_test.go")

KUBIC_INIT_GEN_SRCS       = $(shell grep -l -r "//go:generate" $(SOURCES_DIRS))
KUBIC_INIT_CRD_TYPES_SRCS = $(shell find pkg/apis/kubic -type f -name "*_types.go")

KUBIC_INIT_EXE  = cmd/kubic-init/kubic-init
KUBIC_INIT_MAIN = cmd/kubic-init/main.go
KUBIC_INIT_CFG  = $(CURDIR)/config/kubic-init.yaml
.DEFAULT_GOAL: $(KUBIC_INIT_EXE)

IMAGE_BASENAME = $(GOPATH_THIS_REPO)
IMAGE_NAME     = $(GOPATH_THIS_USER)/$(IMAGE_BASENAME)
IMAGE_TAR_GZ   = $(IMAGE_BASENAME)-latest.tar.gz
IMAGE_DEPS     = $(KUBIC_INIT_EXE) $(KUBIC_INIT_CFG) Dockerfile

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
TF_ARGS_DEFAULT      = -input=false -auto-approve -var 'kubic_init_image=$(IMAGE_TAR_GZ)'

# sudo command (and version passing env vars)
SUDO = sudo
SUDO_E = $(SUDO) -E

# the kubeconfig program generated by kubeadm/kube-init
KUBECONFIG = /etc/kubernetes/admin.conf

# increase to 8 for detailed kubeadm logs...
# Example: make local-run VERBOSE_LEVEL=8
VERBOSE_LEVEL = 5

CONTAINER_VOLUMES = \
		-v $(KUBIC_INIT_CFG):/etc/kubic/kubic-init.yaml \
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

dep-ensure: dep-exe Gopkg.toml
	@echo ">>> Checking vendored deps (respecting Gopkg.toml constraints)"
	dep ensure -v && dep status

dep-update: dep-exe Gopkg.toml
	@echo ">>> Updating vendored deps (respecting Gopkg.toml constraints)"
	dep ensure -update -v && dep status

# download automatically the vendored deps when "vendor" doesn't exist
vendor: dep-exe
	@[ -d vendor ] || dep ensure -v

generate: $(KUBIC_INIT_GEN_SRCS)
	@echo ">>> Generating files..."
	@go generate -x $(SOURCES_DIRS_GO)

# Create a new CRD object XXXXX with:
#    kubebuilder create api --namespaced=false --group kubic --version v1beta1 --kind XXXXX


manifests-rbac: $(KUBIC_INIT_CRD_TYPES_SRCS)
	@echo ">>> Creating RBAC manifests..."
	rm -rf config/rbac/*.yaml
	go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go rbac --name "kubic:manager"

manifests-crd: $(KUBIC_INIT_CRD_TYPES_SRCS)
	@echo ">>> Creating CRDs manifests..."
	rm -rf config/crds/*.yaml
	go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go crd --domain "opensuse.org"

manifests: manifests-rbac manifests-crd

$(KUBIC_INIT_EXE): $(KUBIC_INIT_MAIN_SRCS) generate Gopkg.lock vendor
	@echo ">>> Building $(KUBIC_INIT_EXE)..."
	go build $(KUBIC_INIT_LDFLAGS) -o $(KUBIC_INIT_EXE) $(KUBIC_INIT_MAIN)

.PHONY: fmt
fmt: $(KUBIC_INIT_SRCS)
	@echo ">>> Reformatting code"
	@go fmt $(SOURCES_DIRS_GO)

.PHONY: simplify
simplify:
	@gofmt -s -l -w $(KUBIC_INIT_SRCS)

.PHONY: check
check:
	@test -z $(shell gofmt -l $(KUBIC_INIT_MAIN) | tee /dev/stderr) || echo "[WARN] Fix formatting issues with 'make fmt'"
	@for d in $$(go list ./... | grep -v /vendor/); do golint $${d}; done
	@go tool vet ${KUBIC_INIT_SRCS}

.PHONY: test
test:
	@go test -v ./pkg/... ./cmd/... -coverprofile cover.out

.PHONY: check
clean: docker-reset kubelet-reset docker-image-clean
	rm -f $(KUBIC_INIT_EXE)
	rm -f config/rbac/*.yaml config/crds/*.yaml

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
	$(SUDO_E) $(KUBIC_INIT_EXE) reset -v $(VERBOSE_LEVEL)


# Usage:
#  - create a local seeder: make local-run
#  - create a local seeder with a specific token: TOKEN=XXXX make local-run
#  - join an existing seeder: env SEEDER=1.2.3.4 TOKEN=XXXX make local-run
#
# You can customize the args with something like:
#   make local-run VERBOSE_LEVEL=8 \
#                  KUBIC_INIT_CFG="my-config-file.yaml" \
#                  KUBIC_ARGS="--var Runtime.Engine=docker"
#
local-run: $(KUBIC_INIT_EXE) $(KUBE_DROPIN_DST) local-reset
	@echo ">>> Running $(KUBIC_INIT_EXE) as _root_"
	$(SUDO_E) $(KUBIC_INIT_EXE) bootstrap \
		-v $(VERBOSE_LEVEL) \
		--config $(KUBIC_INIT_CFG) \
		--deploy-manager=false \
		--load-assets=false $(KUBIC_ARGS)

# Usage:
# - Run it locally:
#   make local-run-manager KUBIC_INIT_CFG=test.yaml VERBOSE_LEVEL=5
# - Start a Deployment with the manager:
#   make local-run-manager KUBIC_ARGS="--deployment"
#
local-run-manager: $(KUBIC_INIT_EXE) manifests
	[ -r $(KUBECONFIG) ] || $(SUDO_E) chmod 644 $(KUBECONFIG)
	@echo ">>> Running $(KUBIC_INIT_EXE) as _root_"
	$(KUBIC_INIT_EXE) manager \
		-v $(VERBOSE_LEVEL) \
		--config $(KUBIC_INIT_CFG) \
		--kubeconfig /etc/kubernetes/admin.conf \
		--load-assets=true \
		--crds-dir config/crds \
		--rbac-dir config/rbac \
		$(KUBIC_ARGS)

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
		$(IMAGE_NAME):latest $(KUBIC_ARGS)

docker-reset: kubeadm-reset

$(IMAGE_TAR_GZ): $(IMAGE_DEPS) manifests
	@echo ">>> Creating Docker image..."
	docker build \
		--build-arg KUBIC_INIT_EXE=$(KUBIC_INIT_EXE) \
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
		$(IMAGE_NAME):latest $(KUBIC_ARGS)

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

#
# Usage:
# - create a only-one-seeder cluster:
#   $ make tf-full-run TF_ARGS="-var nodes_count=0"
#
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
	-make tf-full-apply TF_ARGS="-var nodes_count=0 $(TF_ARGS)"

tf-seeder-reapply:
	-make tf-full-reapply TF_ARGS="-var nodes_count=0 $(TF_ARGS)"

tf-seeder-destroy:
	-make tf-full-destroy TF_ARGS="-var nodes_count=0 $(TF_ARGS)"

tf-seeder-nuke: tf-full-nuke

### Terraform only-nodes deployment

$(TF_LIBVIRT_NODES_DIR)/.terraform:
	cd $(TF_LIBVIRT_NODES_DIR) && terraform init

tf-nodes-plan: $(TF_LIBVIRT_NODES_DIR)/.terraform
	cd $(TF_LIBVIRT_NODES_DIR) && terraform plan

#
# Usage:
# - create only one node (ie, for connecting to the seeder started locally with `make local-run`):
#   $ env TOKEN=XXXX make tf-nodes-run
#
tf-nodes-run: tf-nodes-apply
tf-nodes-apply: $(TF_LIBVIRT_NODES_DIR)/.terraform $(IMAGE_TAR_GZ)
	@echo ">>> Deploying only-nodes with Terraform..."
	cd $(TF_LIBVIRT_NODES_DIR) && terraform init && terraform apply $(TF_ARGS_DEFAULT) $(TF_ARGS)

tf-nodes-reapply:
	cd $(TF_LIBVIRT_NODES_DIR) && terraform init && terraform apply $(TF_ARGS_DEFAULT) $(TF_ARGS)

tf-nodes-destroy: $(TF_LIBVIRT_NODES_DIR)/.terraform
	cd $(TF_LIBVIRT_NODES_DIR) && terraform init && terraform destroy -force $(TF_ARGS_DEFAULT) $(TF_ARGS)

tf-nodes-nuke:
	-make tf-nodes-destroy
	cd $(TF_LIBVIRT_NODES_DIR) && rm -f *.tfstate*

#############################################################
# Other stuff
#############################################################
