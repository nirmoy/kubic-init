IMAGE_BASENAME = kubic-init
IMAGE_NAME     = kubic-project/$(IMAGE_BASENAME)
IMAGE_TAR_GZ   = $(IMAGE_BASENAME)-latest.tar.gz
IMAGE_DEPS     = $(KUBIC_INIT_EXE) Dockerfile

# These will be provided to the target
KUBIC_INIT_VERSION    := 1.0.0
KUBIC_INIT_BUILD      := `git rev-parse HEAD 2>/dev/null`
KUBIC_INIT_BRANCH     := $(shell git rev-parse --abbrev-ref HEAD 2> /dev/null  || echo 'unknown')
KUBIC_INIT_BUILD_DATE := $(shell date +%Y%m%d-%H:%M:%S)

# extra args to add to the `kubic-init bootstrap` command
KUBIC_INIT_EXTRA_ARGS =

MANIFEST_LOCAL = deployments/kubelet/kubic-init-manifest.yaml
MANIFEST_REM   = deployments/kubic-init-manifest.yaml
MANIFEST_DIR   = /etc/kubernetes/manifests

KUBE_DROPIN_SRC = init/kubelet.drop-in.conf
KUBE_DROPIN_DST = /etc/systemd/system/kubelet.service.d/kubelet.drop-in.conf

# sudo command (and version passing env vars)
SUDO = sudo
SUDO_E = $(SUDO) -E

# the kubeconfig program generated by kubeadm/kube-init
KUBECONFIG = /etc/kubernetes/admin.conf

# the initial kubelet config
SYS_KUBELET_CONFIG   := /etc/kubernetes/kubelet-config.yaml
LOCAL_KUBELET_CONFIG := init/kubelet-config.yaml
KUBELET_CONFIG       := $(shell [ -f $(SYS_KUBELET_CONFIG) ] && echo $(SYS_KUBELET_CONFIG) || echo $(LOCAL_KUBELET_CONFIG) )

# increase to 8 for detailed kubeadm logs...
# Example: make local-run VERBOSE_LEVEL=8
VERBOSE_LEVEL = 3

# volumes to mount when running locally
CONTAINER_VOLUMES = \
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
# Some simple run targets
# (for testing things locally)
#############################################################

# we must "patch" the local kubelet by adding a drop-in unit
# otherwise, the kubelet will be run with the wrong arguments
/var/lib/kubelet/config.yaml: $(KUBELET_CONFIG)
	$(SUDO) cp -f $(KUBELET_CONFIG) /var/lib/kubelet/config.yaml

$(KUBE_DROPIN_DST): $(KUBE_DROPIN_SRC) /var/lib/kubelet/config.yaml
	@echo ">>> Adding drop-in unit for the local kubelet"
	$(SUDO) mkdir -p `dirname $(KUBE_DROPIN_DST)`
	$(SUDO) cp -f $(KUBE_DROPIN_SRC) $(KUBE_DROPIN_DST)
	$(SUDO) systemctl daemon-reload

kubeadm-reset: local-reset
local-reset: $(KUBIC_INIT_EXE)
	@echo ">>> Resetting everything..."
	$(SUDO_E) $(KUBIC_INIT_EXE) reset \
		-v $(VERBOSE_LEVEL) \
		$(KUBIC_INIT_EXTRA_ARGS)


# Usage:
#  - create a local seeder:
#    $ make local-run
#  - create a local seeder with a specific token:
#    $ env TOKEN=XXXX make local-run
#  - join an existing seeder:
#    $ env SEEDER=1.2.3.4 TOKEN=XXXX make local-run
#  - run a custom kubeadm, use docker, our own configuration and a higher debug level:
#    $ make local-run \
#     KUBIC_INIT_EXTRA_ARGS="--config my-config-file.yaml --var Runtime.Engine=docker --var Paths.Kubeadm=/somewhere/linux/amd64/kubeadm" \
#     VERBOSE_LEVEL=8
#
# You can customize the args with something like:
#   make local-run VERBOSE_LEVEL=8 \
#                  KUBIC_INIT_EXTRA_ARGS="--config my-config-file.yaml --var Runtime.Engine=docker"
#
local-run: $(KUBIC_INIT_EXE) $(KUBE_DROPIN_DST)
	@echo ">>> Running $(KUBIC_INIT_EXE) as _root_"
	$(SUDO_E) $(KUBIC_INIT_EXE) bootstrap \
		-v $(VERBOSE_LEVEL) \
		--load-assets=false \
		$(KUBIC_INIT_EXTRA_ARGS)

# Usage:
#  - create a local seeder: make docker-run
#  - create a local seeder with a specific token: TOKEN=XXXX make docker-run
#  - join an existing seeder: env SEEDER=1.2.3.4 TOKEN=XXXX make docker-run
docker-run: $(IMAGE_TAR_GZ) $(KUBE_DROPIN_DST)
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
		$(IMAGE_NAME):latest $(KUBIC_INIT_EXTRA_ARGS)

docker-reset: kubeadm-reset

$(IMAGE_TAR_GZ): $(IMAGE_DEPS)
	@echo ">>> Creating Docker image..."
	docker build -t $(IMAGE_NAME):latest .
	@echo ">>> Creating tar for image..."
	docker save $(IMAGE_NAME):latest | gzip > $(IMAGE_TAR_GZ)

docker-image: $(IMAGE_TAR_GZ)
docker-image-clean:
	rm -f $(IMAGE_TAR_GZ)
	-docker rmi $(IMAGE_NAME)

# TODO: build the image for podman
# TODO: implement podman-reset
podman-run: podman-image $(KUBE_DROPIN_DST)
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
		$(IMAGE_NAME):latest $(KUBIC_INIT_EXTRA_ARGS)

kubelet-run: $(IMAGE_TAR_GZ) $(KUBE_DROPIN_DST)
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
	@echo ">>> Restoring a safe kubelet configuration..."
	$(SUDO) cp -f $(KUBELET_CONFIG) /var/lib/kubelet/config.yaml
	@-rm -f $(MANIFEST_DIR)/$(MANIFEST_REM)

