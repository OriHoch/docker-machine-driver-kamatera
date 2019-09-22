# Contributing to docker-machine-driver-kamatera

* Welcome to Kamatera!
* Contributions of any kind are welcome.

## Building from source

Use an up-to-date version of [Go](https://golang.org/dl) and [dep](https://github.com/golang/dep)

Download the sources

```
go get github.com/OriHoch/docker-machine-driver-kamatera
```

Set some go environment variables

```
export GOPATH=$(go env GOPATH)
export GOBIN=$GOPATH/bin
export PATH="$PATH:$GOBIN"
```

Change to the project directory

```
cd $GOPATH/src/github.com/OriHoch/docker-machine-driver-kamatera/
```

Build

```
go build -o docker-machine-driver-kamatera
```

Set your Kamatera api keys in environment variables

```
export KAMATERA_API_CLIENT_ID=
export KAMATERA_API_SECRET=
```

Add current directory to PATH so that docker-machine can find the driver binary:

```
export PATH=`pwd`:$PATH
```

Create a machine

```
docker-machine --debug create -d kamatera my-server
```

## Run tests

The test creates, tests and deletes a machine

Copy the binary to the tests directory

```
cp -f `which docker-machine-driver-kamatera` tests/
```

Run a test

```
python3.6 tests/test.py
```

Run multiple tests and aggregate statistics and results

```
RESULTS_DIRECTORY=`pwd`/test_results

# a unique title for this test suite run
export SUITE_RUN_TITLE="kamatera-suite-1"

# test settings
export NUM_SINGLE_MACHINE_TESTS_TO_RUN=50
export MAX_PARALLEL_SINGLE_MACHINE_TESTS=10

# tests an account which is limited to a 500 USD Server, maximum allowed servers: 5, server create frequency: unlimited.
export TEST_ACCOUNT=PRICELIMIT

docker build -t tests tests/ &&\
docker run -it \
           -v /var/run/docker.sock:/var/run/docker.sock \
           -v "${RESULTS_DIRECTORY}/${SUITE_RUN_TITLE}/:/test_results/" \
           -e KAMATERA_API_CLIENT_ID \
           -e KAMATERA_API_SECRET \
           -e "KAMATERA_HOST_PATH=${RESULTS_DIRECTORY}/${SUITE_RUN_TITLE}" \
           -e SUITE_RUN_TITLE -e NUM_SINGLE_MACHINE_TESTS_TO_RUN -e MAX_PARALLEL_SINGLE_MACHINE_TESTS \
           -e TEST_ACCOUNT \
           tests tests_suite.py
```

Test using pre-created machines

```
export TEST_EXISTING_MACHINES="comma-separated-list-of-machine-names"

docker build -t tests tests/ &&\
docker run -it \
           -v /var/run/docker.sock:/var/run/docker.sock \
           -v "${HOME}/.docker:${HOME}/.docker" \
           -v "${HOME}/.docker:/root/.docker" \
           -v "${RESULTS_DIRECTORY}/${SUITE_RUN_TITLE}/:/test_results/" \
           -e KAMATERA_API_CLIENT_ID \
           -e KAMATERA_API_SECRET \
           -e "KAMATERA_HOST_PATH=${RESULTS_DIRECTORY}/${SUITE_RUN_TITLE}" \
           -e SUITE_RUN_TITLE -e NUM_SINGLE_MACHINE_TESTS_TO_RUN -e MAX_PARALLEL_SINGLE_MACHINE_TESTS \
           -e TEST_EXISTING_MACHINES \
           -e TEST_HOST_DOCKERDIR="${HOME}/.docker" \
           tests tests_suite.py
```

While tests are running you can follow the individual test logs:

```
tail -f $RESULTS_DIRECTORY/${SUITE_RUN_TITLE}/test1/logs
```

Cleanup all the test servers in the Kamatera account

```
python3.6 tests/cleanup.py "ktm-"
```
