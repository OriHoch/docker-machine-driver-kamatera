#!/usr/bin/env python3.6
import os
import subprocess
import binascii
import datetime
import traceback
import time


# run a single machine test of various operations
# prints debug output and exits with returncode 0 = success, 1 = failure


TEST_PLAN = [
    # create
    ('create', {'returncode': 0}),
    ('hello-world', {'returncode': 0}),

    # restart
    ('restart', {'returncode': 0}),
    ('hello-world', {'returncode': 0}),

    # stop
    ('stop', {'returncode': 0}),
    ('hello-world', {'returncode': -1}),

    # start
    ('start', {'returncode': 0}),
    ('hello-world', {'returncode': 0}),

    # kill
    ('kill', {'returncode': 0}),
    ('hello-world', {'returncode': -1}),

    # restsart after kill
    ('restart', {'returncode': 0}),
    ('hello-world', {'returncode': 0}),

    # ssh
    ('ssh-hostname', {'returncode': 0, 'output': '{machine_name}'}),

    # remove
    ('rm', {'returncode': 0}),
    ('hello-world', {'returncode': -1}),
]



if not os.environ.get('KAMATERA_API_CLIENT_ID') or not os.environ.get('KAMATERA_API_SECRET'):
    print('Missing required env vars: KAMATERA_API_CLIENT_ID KAMATERA_API_SECRET')
    exit(1)


# 40 minutes - timeout for the create command
KAMATERA_CREATE_TIMEOUT_SECONDS = int(os.environ.get('KAMATERA_CREATE_TIMEOUT_SECONDS', '2400'))


# 20 minutes - timeout for power operations (restart, stop, start etc..)
KAMATERA_POWER_TIMEOUT_SECONDS = int(os.environ.get('KAMATERA_POWER_TIMEOUT_SECONDS', '1200'))


# 20 minute - default timeout for all other test commands
KAMATERA_DEFAULT_TIMEOUT_SECONDS = int(os.environ.get('KAMATERA_DEFAULT_TIMEOUT_SECONDS', '1200'))


# global timeout for all commands, defaults to 0 which uses a sum of the individual command timeouts
KAMATERA_GLOBAL_TIMEOUT_SECONDS = int(os.environ.get('KAMATERA_GLOBAL_TIMEOUT_SECONDS', '0'))


# optional prefix for test machine, default to ktm-YYMMDD (ktm = kamatera test machine)
# all machine names are suffixed with unique id, so prefix is only for convenience
KAMATERA_TEST_MACHINE_PREFIX = os.environ.get('KAMATERA_TEST_MACHINE_PREFIX',
                                              'ktm-{}'.format(datetime.datetime.now().strftime('%Y%m%d')))


KAMATERA_DOCKER_MACHINE_PATH_TEMPLATE = os.environ.get('KAMATERA_DOCKER_MACHINE_PATH_TEMPLATE', '~/.docker/machine/machines/{machine_name}')
KAMATERA_DOCKER_MACHINE_PATH_TEMPLATE = os.path.expanduser(KAMATERA_DOCKER_MACHINE_PATH_TEMPLATE)


KAMATERA_TEST_CREATED_MACHINE_NAME = os.environ.get('KAMATERA_TEST_CREATED_MACHINE_NAME', '')

# how many errors are encountered to stop the test
# defaults to 1 - test stops on first error
KAMATERA_MAX_ERRORS_TO_STOP = int(os.environ.get('KAMATERA_MAX_ERRORS_TO_STOP', '1'))


if KAMATERA_TEST_CREATED_MACHINE_NAME:
    machine_name = KAMATERA_TEST_CREATED_MACHINE_NAME
else:
    machine_name = '{}-{}'.format(KAMATERA_TEST_MACHINE_PREFIX, binascii.hexlify(os.urandom(8)).decode())


def info(*args):
    print('\n', '[[ {} {} ]]'.format(machine_name, datetime.datetime.now().strftime('%Y-%m-%d %H:%M:%S')),
          '\n', *args, '\n')


def get_machine_host():
    p = subprocess.run(['docker-machine', 'url', machine_name], stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                       timeout=KAMATERA_DEFAULT_TIMEOUT_SECONDS)
    returncode = p.returncode
    output = p.stdout.decode()
    if returncode != 0:
        print(output)
        return None
    else:
        return output.strip()


test_plan_info_args = ['Test Plan:\n']
global_timeout_seconds = 0
for test_num, test in enumerate(TEST_PLAN):
    if test[0] == 'create':
        timeout = KAMATERA_CREATE_TIMEOUT_SECONDS
    elif test[0] in ['restart', 'stop', 'start', 'kill', 'rm']:
        timeout = KAMATERA_POWER_TIMEOUT_SECONDS
    else:
        timeout = KAMATERA_DEFAULT_TIMEOUT_SECONDS
    test[1]['timeout'] = timeout
    global_timeout_seconds += timeout
    test_plan_info_args += [test_num, '{}\n'.format(test)]
info(*test_plan_info_args)

if KAMATERA_GLOBAL_TIMEOUT_SECONDS > 0:
    global_timeout_seconds = KAMATERA_GLOBAL_TIMEOUT_SECONDS


info('KAMATERA_MAX_ERRORS_TO_STOP = {}\n'.format(KAMATERA_MAX_ERRORS_TO_STOP),
     'global timeout (seconds) = {}\n'.format(global_timeout_seconds),
     '--- Starting test ---')


global_start_datetime = datetime.datetime.now()


def get_cmd(cmd, assertions):
    env = None
    is_machine = True
    max_retries = 1
    if cmd == 'hello-world':
        cmd = ['docker', 'run', 'hello-world']
        machine_host = get_machine_host()
        max_retries = 10
        if not machine_host:
            return None, None, max_retries
        env = {'DOCKER_TLS_VERIFY': '1',
               'DOCKER_HOST': machine_host,
               'DOCKER_CERT_PATH': KAMATERA_DOCKER_MACHINE_PATH_TEMPLATE.format(machine_name=machine_name),
               'DOCKER_MACHINE_NAME': machine_name}
        is_machine = False
    elif cmd == 'create':
        cmd = ['create', '-d', 'kamatera', machine_name]
    elif cmd == 'ssh-hostname':
        cmd = ['ssh', machine_name, 'hostname']
    elif cmd == 'rm':
        cmd = ['rm', '-y', machine_name]
    elif cmd == 'rm-f':
        cmd = ['rm', '-y', '-f', machine_name]
    else:
        cmd = [cmd, machine_name]
    if is_machine:
        extra_args = []
        if 'output' not in assertions:
            extra_args.append('--debug')
        cmd = ['docker-machine', *extra_args, *cmd]
    return cmd, env, max_retries


def run_cmd(cmd, env, assertions, cmd_timeout_seconds):
    info('Running cmd:', cmd, env, assertions, cmd_timeout_seconds)
    try:
        if 'output' in assertions:
            p = subprocess.run(cmd, timeout=cmd_timeout_seconds, env=env,
                               stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
            returncode = p.returncode
            output = p.stdout.decode().strip()
        else:
            p = subprocess.run(cmd, timeout=cmd_timeout_seconds, env=env)
            returncode = p.returncode
            output, stderr = '', ''
    except subprocess.TimeoutExpired:
        print('command timeout expired, returncode = 255')
        return 255, ''
    except Exception as e:
        print('unexpected exception, returncode = 244: {} {}'.format(traceback.format_exc(), e))
        return 244, ''
    if returncode != 0:
        print(output)
    print('subprocess completed, returncode =', returncode)
    if len(output) > 0:
        print('len(output)={}'.format(len(output)))
    return returncode, output


def run_test(test_num, test, errors, num_retries=0):
    cmd, assertions = test
    if cmd == 'create' and KAMATERA_TEST_CREATED_MACHINE_NAME:
        info('Skipping create, using existing machine')
        return
    info('Starting test', test_num, cmd, assertions)
    cmd, env, max_retries = get_cmd(cmd, assertions)
    _errors = []
    if cmd is None:
        if {k:v for k,v in assertions.items() if k != 'timeout'} == {'returncode': -1}:
            # assertion returncode = -1 means process is expected to fail
            return
        else:
            _errors.append('({}) failed: cmd = "{}"'.format(test_num, cmd))
            print(_errors[-1])
    else:
        cmd_timeout_seconds = assertions.pop('timeout')
        returncode, output = run_cmd(cmd, env, assertions, cmd_timeout_seconds)
        for assertion, expected_value in assertions.items():
            if assertion == 'returncode':
                if (expected_value < 0 and returncode == 0) or (expected_value > -1 and returncode != expected_value):
                    _errors.append(
                        '({}) failed: cmd = "{}", assertion = "{}", expected = "{}", actual = "{}"'.format(
                            test_num, cmd, assertion, expected_value, returncode
                        )
                    )
                    print(_errors[-1])
            elif assertion == 'output':
                assert type(expected_value) == str
                expected_value = expected_value.format(machine_name=machine_name)
                if output != expected_value:
                    _errors.append(
                        '({}) failed: cmd = "{}", assertion = "{}", expected = "{}", actual = "{}"'.format(
                            test_num, cmd, assertion, expected_value, output
                        )
                    )
                    print(_errors[-1])
            else:
                raise Exception('invalid assertion: {} = {}'.format(assertion, expected_value))
    if len(_errors) > 0:
        if max_retries > 1 and num_retries < max_retries:
            print(_errors)
            print('Retrying failed test.. {}/{}'.format(num_retries, max_retries))
            time.sleep(1*num_retries)
            return run_test(test_num, test, errors, num_retries + 1)
        else:
            errors += _errors
            if KAMATERA_MAX_ERRORS_TO_STOP > 0 and len(errors) >= KAMATERA_MAX_ERRORS_TO_STOP:
                print('Too many errors! ({})'.format(len(errors)))
                print('------------ failed test summary ------------')
                for e in errors:
                    print(' > ' + e)
                exit(1)


def run_tests():
    errors = []
    for test_num, test in enumerate(TEST_PLAN):
        test_start_datetime = datetime.datetime.now()
        run_test(test_num, test, errors)
        test_finish_datetime = datetime.datetime.now()
        test_elapsed_seconds = (test_finish_datetime - test_start_datetime).seconds
        global_elapsed_seconds = (test_finish_datetime - global_start_datetime).seconds
        print('test elapsed seconds:', test_elapsed_seconds)
        print('global elapsed seconds:', global_elapsed_seconds)
        if global_elapsed_seconds > global_timeout_seconds:
            print('exceeded global timeout')
            exit(1)
        print('test', test_num, 'completed')
    global_elapsed_seconds = (datetime.datetime.now() - global_start_datetime).seconds
    info('All tests completed in {} seconds'.format(global_elapsed_seconds))
    if len(errors) > 0:
        print('Encountered {} errors:'.format(len(errors)))
        for e in errors:
            print(' > ' + e)
        print('\nTest Failed!')
        exit(1)
    else:
        print('\nGreat Success!')
        exit(0)


run_tests()
