#############################################################
# End To End Tests
#############################################################

SEEDER := $(shell cd $(TF_LIBVIRT_FULL_DIR) && terraform output -json seeder 2>/dev/null | python -c 'import sys, json; print json.load(sys.stdin)["value"]' 2>/dev/null || echo "unknown")
tf-e2e-tests:
	cd tests && SEEDER=$(SEEDER) ./run_suites.sh

