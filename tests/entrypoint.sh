#!/usr/bin/env bash

if [ "${1}" == "" ]; then
    # Test a single machine and delete when test completes
    # test results are available in the following files (inside the container):
    # /kamatera/status - updated on test completion to "OK" or "ERROR" (as well as returning relevant exit code)
    # /kamatera/logs - all log output is streamed to this file
    mkdir /kamatera
    echo Starting single machine test > /kamatera/logs
    echo $(date +%Y-%m-%d) $(date +%H:%M:%S%N) >> /kamatera/logs
    echo "" > /kamatera/status
    if /test.py >> /kamatera/logs 2>&1; then
        echo $(date +%Y-%m-%d) $(date +%H:%M:%S%N) >> /kamatera/logs
        echo Great Success! >> /kamatera/logs
        echo OK > /kamatera/status
        exit 0
    else
        echo $(date +%Y-%m-%d) $(date +%H:%M:%S%N) >> /kamatera/logs
        echo "test failed" >> /kamatera/logs
        echo ERROR > /kamatera/status
        exit 1
    fi

elif [ "${1}" == "bash" ]; then
    exec bash

else
    # Run other Python test scripts (e.g. tests_suite.py)
    exec python3.6 /"${1}"

fi
