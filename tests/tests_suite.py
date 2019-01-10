#!/usr/bin/env python3.6
import os
import subprocess
import time
import csv


# Runs a full tests suite using Docker to run and track multiple test instances in parallel
# All test results are aggregated and stored in /test_results/results.csv
# Detailed results for each test are under /test_results/test<TEST_NUM>/(status|logs)


NUM_SINGLE_MACHINE_TESTS_TO_RUN = int(os.environ.get('NUM_SINGLE_MACHINE_TESTS_TO_RUN', '2'))


if not os.environ.get('KAMATERA_API_CLIENT_ID') or not os.environ.get('KAMATERA_API_SECRET'):
    print('Missing required env vars: KAMATERA_API_CLIENT_ID KAMATERA_API_SECRET')
    exit(1)


if not os.environ.get('KAMATERA_HOST_PATH') or not os.environ.get('SUITE_RUN_TITLE'):
    print('Missing KAMATERA_HOST_PATH and SUITE_RUN_TITLE environment variables')
    exit(1)

host_path = os.environ['KAMATERA_HOST_PATH']
local_path = '/test_results'
suite_run_title = os.environ['SUITE_RUN_TITLE']

print('Running tests suite {}'.format(suite_run_title))
print(' -- NUM_SINGLE_MACHINE_TESTS_TO_RUN={}'.format(NUM_SINGLE_MACHINE_TESTS_TO_RUN))
print(' -- host_path={}'.format(host_path))
print(' -- local_path={}'.format(local_path))

i = 0
while i < NUM_SINGLE_MACHINE_TESTS_TO_RUN:
    i += 1
    print('Running test {} / {}'.format(i, NUM_SINGLE_MACHINE_TESTS_TO_RUN))
    subprocess.check_call("""
        docker run --rm --name {suite_run_title}-test{i} -d \
            -v {kamatera_host_path}/test{i}/:/kamatera/ \
            -e TESTS_DEBUG=1 \
            -e KAMATERA_API_CLIENT_ID \
            -e KAMATERA_API_SECRET \
            tests
    """.format(i=i, kamatera_host_path=host_path, suite_run_title=suite_run_title), shell=True)

print("Waiting for tests to complete...")

test_status = {}
while len(test_status) != NUM_SINGLE_MACHINE_TESTS_TO_RUN:
    time.sleep(5)
    print('.')
    i = 0
    while i < NUM_SINGLE_MACHINE_TESTS_TO_RUN:
        i += 1
        if 'test{}'.format(i) in test_status: continue
        status_path = '{}/test{}/status'.format(local_path, i)
        if os.path.exists(status_path):
            with open(status_path) as f:
                status = f.read().strip()
            if status in ['OK', 'ERROR']:
                test_status['test{}'.format(i)] = status
                print('test{}: {}'.format(i, status))
                print('completed tests: {} / {}'.format(len(test_status), NUM_SINGLE_MACHINE_TESTS_TO_RUN))
            elif status != '':
                raise Exception('Invalid status: {}'.format(status))

success_tests = [test_name for test_name, status in test_status.items() if status == 'OK']
errored_tests = [test_name for test_name, status in test_status.items() if status == 'ERROR']

print('** {} Successfull tests **'.format(len(success_tests)))
for test_name in success_tests:
    print('  -- {}: OK'.format(test_name))

print('** {} Failed tests **'.format(len(errored_tests)))
for test_name in errored_tests:
    print('  -- {}: ERROR'.format(test_name))
    print(' ----- last 30 log lines ----- ')
    subprocess.check_call('tail -10 {}/{}/logs'.format(local_path, test_name), shell=True)
    print(' ----- end of last 30 log lines ({}: ERROR) ----- '.format(test_name))

print("Aggregating test results...")
with open('{}/results.csv'.format(local_path), 'w') as csvfile:
    csvwriter = csv.writer(csvfile)
    csvwriter.writerow(['test_name', 'status', 'error', 'last_logs'])
    for test_name, status in test_status.items():
        returncode, output = subprocess.getstatusoutput('tail -10 {}/{}/logs'.format(local_path, test_name))
        if returncode == 0:
            last_logs = output
            error = ''
        else:
            last_logs = output
            error = 'failed to get last logs'
        csvwriter.writerow([test_name, status, error, last_logs])

if len(errored_tests) > 0 or len(success_tests) != NUM_SINGLE_MACHINE_TESTS_TO_RUN:
    print('Test suite failed')
    exit(1)
else:
    print('Great Success!')
    exit(0)
