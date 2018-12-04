# End to End tests for Kubic-init

This tests are BDD style, using ginkgo Kubernetes framework for doing specific `kubic-init` e2e tests.

# Testsuites for kubic-init

- cluster-health : this tests will check a `kubic-init` cluster health.

# Prerequisites for e2e tests.

0) you need to have deployed kubic-init. `make tf-full-apply` on root of this dir.

1) Run the following script
```bash
SEEDER=192.168.122.73 ./run_suites.sh 
```

# Envinronment Variables supported

Add here only needed variables. Try to autodectect and be minimalist.

#### Mandatory
`SEEDER`: MANDATORY- This will contain the IP adress of your master

#### Optional
`SEEDER_SSH_PWD`: OPTIONAL- This is the root password for connecting to your master via ssh, by default this is `linux`. Set this to change.

# Architecture and design:

A testsuite is a subdirectory of `tests` and exist conceptually like a indipendent microservice.

The testsuite share only the `lib` directory which are utilty. 
The Common library is stored on `lib` directory, You should try to put code there to make clean the specs.

This testsuite can be executed indipendently from each framework of deployment. You need only the `kubic-init` source code.

You need only to pass the SEEDER as env variable, and you can run the tests to any deployed `kubic-init` cluster outside in the wild.
Alls hosts/vms should have sshd enabled on port 22. We use linux as std password but you can change it with the ENV.variable.

# Developing New Tests:

## Tests requirements:

0) All tests should be idempotent, meanining you can run them XX times, you will have the same results.

1) All tests can be run in parallel.

2) All tests doesn't require or have dependencies each others. Meaining: we can change order in which tests are executed, results will be the same. There is no hidden dependency between tests.

## Run the tests:

```golang
ginkgo -r --randomizeAllSpecs --randomizeSuites --failOnPending --cover --trace --race --progress
```
This will run all sub-suites in random order, random sub-test.

## How to create a new suite:

Generally we should avoid to create much subsuites if they are not needed. 

0) Create a dir like `your_suite_name`
1) Create a pkg accordingly inside the dir. This pkg should be empty, only containing `pkg services` as example.
2) Use `ginkgo bootstrap` for createing the `testsuite` file
3) Use `ginkgo generate name_test` for generating specs. 

See upstream doc for further details.
