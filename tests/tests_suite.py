#!/usr/bin/env python3.6
import os
import subprocess
import time
import csv
import datetime
from create_args_generator import create_args_generator


# Runs a full tests suite using Docker to run and track multiple test instances in parallel
# All test results are aggregated and stored in /test_results/results.csv
# Detailed results for each test are under /test_results/test<TEST_NUM>/(status|logs)


NUM_SINGLE_MACHINE_TESTS_TO_RUN = int(os.environ.get('NUM_SINGLE_MACHINE_TESTS_TO_RUN', '2'))
MAX_PARALLEL_SINGLE_MACHINE_TESTS = int(os.environ.get('MAX_PARALLEL_SINGLE_MACHINE_TESTS', '5'))
TEST_HOST_DOCKERDIR = os.environ.get('TEST_HOST_DOCKERDIR', '')
GLOBAL_TIMEOUT_SECONDS = int(os.environ.get('GLOBAL_TIMEOUT_SECONDS', '900'))  # 15 minutes


TEST_EXISTING_MACHINES = os.environ.get('TEST_EXISTING_MACHINES')
if TEST_EXISTING_MACHINES:
    TEST_EXISTING_MACHINES = TEST_EXISTING_MACHINES.split(',')


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
print(' -- MAX_PARALLEL_SINGLE_MACHINE_TESTS={}'.format(MAX_PARALLEL_SINGLE_MACHINE_TESTS))
print(' -- host_path={}'.format(host_path))
print(' -- local_path={}'.format(local_path))


start_datetime = datetime.datetime.now()
create_args_gen = create_args_generator()


def start_tests_batch(test_names, machine_names=None):
    print('Starting test batch: {}'.format(test_names))
    for i, test_name in enumerate(test_names):
        print('Starting {} --- {} / {} in current batch'.format(test_name, i+1, len(test_names)))
        if machine_names:
            extra_args = ' -e KAMATERA_TEST_CREATED_MACHINE_NAME={} '.format(machine_names[i])
        else:
            extra_args = ''
        if TEST_HOST_DOCKERDIR:
            extra_args += ' -v {host_dockerdir}:/root/.docker -v {host_dockerdir}:{host_dockerdir} '.format(
                host_dockerdir=TEST_HOST_DOCKERDIR
            )
        cmd = """docker run --rm --name {suite_run_title}-{test_name} -d \
                            -v {kamatera_host_path}/{test_name}/:/kamatera/ \
                            -e KAMATERA_API_CLIENT_ID \
                            -e KAMATERA_API_SECRET \
                            -e KAMATERA_CREATE_ARGS={create_args} \
                            {extra_args} \
                            tests
              """.format(test_name=test_name, kamatera_host_path=host_path, suite_run_title=suite_run_title,
                       extra_args=extra_args, create_args=','.join(next(create_args_gen)))
        print(cmd)
        subprocess.check_call(cmd, shell=True)
    print('Waiting for test batch to complete')
    batch_test_status = {}
    while len(batch_test_status) != len(test_names):
        time.sleep(10)
        print('.')
        for test_name in test_names:
            if test_name in batch_test_status: continue
            if (datetime.datetime.now() - start_datetime).seconds > GLOBAL_TIMEOUT_SECONDS:
                batch_test_status[test_name] = 'TIMEOUT'
                continue
            status_path = '{}/{}/status'.format(local_path, test_name)
            if os.path.exists(status_path):
                with open(status_path) as f:
                    status = f.read().strip()
                if status in ['OK', 'ERROR']:
                    batch_test_status[test_name] = status
                    print('{}: {}'.format(test_name, status))
                    print('completed tests in current batch: {} / {}'.format(len(batch_test_status), len(test_names)))
                elif status != '':
                    raise Exception('Invalid status: {}'.format(status))
    return batch_test_status


test_status = {}


if TEST_EXISTING_MACHINES:
    test_names = ['test{}'.format(i) for i, machine_name in enumerate(TEST_EXISTING_MACHINES)]
    for test_name, status in start_tests_batch(test_names, TEST_EXISTING_MACHINES).items():
        test_status[test_name] = status
else:
    i = 0
    current_batch = []
    while i < NUM_SINGLE_MACHINE_TESTS_TO_RUN:
        i += 1
        current_batch.append('test{}'.format(i))
        if len(current_batch) >= MAX_PARALLEL_SINGLE_MACHINE_TESTS:
            for test_name, status in start_tests_batch(current_batch).items():
                test_status[test_name] = status
            current_batch = []
    if len(current_batch) > 0:
        for test_name, status in start_tests_batch(current_batch).items():
            test_status[test_name] = status

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

def get_test_results(_test_name):
    returncode, output = subprocess.getstatusoutput('tail -10 {}/{}/logs'.format(local_path, _test_name))
    if returncode == 0:
        last_logs = output
        is_create_frequency_reached = subprocess.call(
            "cat {}/{}/logs | grep '\"code\":52,'".format(local_path, test_name),
            shell=True
        ) == 0
        if is_create_frequency_reached:
            error = 'reached server create frequency limit'
            if os.environ.get('TEST_ACCOUNT') == 'FREQLIMIT':
                error = '{}. Test is not condidered a failure because test account has limited frequency'.format(error)
        else:
            error = ''
    else:
        is_create_frequency_reached = False
        last_logs = output
        error = 'failed to get last logs'
    return error, last_logs, is_create_frequency_reached

print("Aggregating test results...")
num_errors = 0
num_success = 0
with open('{}/results.csv'.format(local_path), 'w') as csvfile:
    csvwriter = csv.writer(csvfile)
    csvwriter.writerow(['test_name', 'status', 'error', 'last_logs'])
    for test_name, status in test_status.items():
        test_error, test_last_logs, is_create_frequency_reached = get_test_results(test_name)
        if is_create_frequency_reached and os.environ.get('TEST_ACCOUNT') == 'FREQLIMIT':
            # Test is not condidered a failure because test account has limited frequency
            num_success += 1
        elif status == 'ERROR':
            num_errors += 1
        else:
            num_success += 1
        csvwriter.writerow([test_name, status, test_error, test_last_logs])


if num_errors > 0 or num_success != NUM_SINGLE_MACHINE_TESTS_TO_RUN:
    print('Test suite failed')
    exit(1)
else:
    print('Great Success!')
    exit(0)
