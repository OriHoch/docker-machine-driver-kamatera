#!/usr/bin/env bash

if [ "${1}" == "" ]; then
    echo usage: tests/run_test_suite.sh NUM_SINGLE_MACHINE_TESTS_TO_RUN
    exit 1
fi

export NUM_SINGLE_MACHINE_TESTS_TO_RUN="${1}"
TEST_RUNNER_SUFFIX="${2}"

! [ -e tests/docker-machine-driver-kamatera ] &&\
    echo please follow CONTRIBUTING.md to build a tests image and run this script from the project root &&\
    exit 1

echo Creating a machine to run the test suite from
! docker-machine --debug create -d kamatera ktm-test-runner${TEST_RUNNER_SUFFIX} && exit 1
eval "$(docker-machine env ktm-test-runner${TEST_RUNNER_SUFFIX})"

echo Building and running the test suite

export RESULTS_DIRECTORY=/kamatera_test${TEST_RUNNER_SUFFIX}_results
export SUITE_RUN_TITLE="kamatera-suite${TEST_RUNNER_SUFFIX}"
export MAX_PARALLEL_SINGLE_MACHINE_TESTS=10
export GLOBAL_TIMEOUT_SECONDS=36000

docker build -t tests tests/ &&\
docker run -d --name test-runner${TEST_RUNNER_SUFFIX} \
           -v /var/run/docker.sock:/var/run/docker.sock \
           -v "${RESULTS_DIRECTORY}/${SUITE_RUN_TITLE}/:/test_results/" \
           -e KAMATERA_API_CLIENT_ID \
           -e KAMATERA_API_SECRET \
           -e "KAMATERA_HOST_PATH=${RESULTS_DIRECTORY}/${SUITE_RUN_TITLE}" \
           -e SUITE_RUN_TITLE -e NUM_SINGLE_MACHINE_TESTS_TO_RUN -e MAX_PARALLEL_SINGLE_MACHINE_TESTS \
           tests tests_suite.py
