#!/usr/bin/env python3.6
import os
import subprocess
import binascii


if not os.environ.get('KAMATERA_API_CLIENT_ID') or not os.environ.get('KAMATERA_API_SECRET'):
    print('Missing required env vars: KAMATERA_API_CLIENT_ID KAMATERA_API_SECRET')
    exit(1)

machine_name = 'test-machine-{}-{}'.format(subprocess.check_output(['date', '+%Y%m%d']).decode().strip(),
                                           binascii.hexlify(os.urandom(8)).decode())
print('test machine name = {}'.format(machine_name))

errors = []

for test_num, test in enumerate([
    # create
    ('create -d kamatera {machine_name}', {'returncode': 0}),
    ('hello-world', {'returncode': 0}),

    # restart
    ('restart {machine_name}', {'returncode': 0}),
    ('hello-world', {'returncode': 0}),

    # stop
    ('stop {machine_name}', {'returncode': 0}),
    ('hello-world', {'returncode': -1}),

    # start
    ('start {machine_name}', {'returncode': 0}),
    ('hello-world', {'returncode': 0}),

    # kill
    ('kill {machine_name}', {'returncode': 0}),
    ('hello-world', {'returncode': -1}),

    # restsart after kill
    ('restart {machine_name}', {'returncode': 0}),
    ('hello-world', {'returncode': 0}),

    # ssh
    ('ssh {machine_name} hostname', {'returncode': 0, 'output': machine_name}),

    # remove
    ('rm -y {machine_name}', {'returncode': 0}),
    ('hello-world', {'returncode': -1}),

    # ensure removal with --force (optional)
    ('rm -f -y {machine_name}', {})
]):
    if test_num > 0:
        print('')
        print(' >>>>>>>> ')
        print('')
        if len(errors) > 0:
            print('Encountered {} errors:'.format(len(errors)))
            for error in errors:
                print(' > ' + error)
        else:
            print('No errors!')
        print('')
        print(' >>>>>>>> ')
    cmd, assertions = test
    if cmd == 'hello-world':
        cmd = 'eval "$(docker-machine env --shell bash {machine_name})" && docker run hello-world'.format(machine_name=machine_name)
    else:
        extra_args = ''
        if os.environ.get('TESTS_DEBUG') and 'output' not in assertions:
            extra_args += ' --debug'
        cmd = 'docker-machine{} {}'.format(extra_args, cmd.format(machine_name=machine_name))
    print('Running cmd: {}'.format(cmd))
    if 'output' in assertions:
        returncode, output = subprocess.getstatusoutput(cmd)
    else:
        returncode, output = subprocess.call(cmd, shell=True), ''
    print('returncode={}'.format(returncode))
    if len(output) > 0:
        print('len(output)={}'.format(len(output)))
    for assertion, expected_value in assertions.items():
        if assertion == 'returncode':
            if (expected_value < 0 and returncode == 0) or (expected_value > -1 and returncode != expected_value):
                errors.append('({}) failed: cmd = "{}", assertion = "{}", expected = "{}", actual = "{}"'.format(test_num, cmd, assertion, expected_value, returncode))
                print(errors[-1])
        elif assertion == 'output':
            if output != expected_value:
                errors.append('({}) failed: cmd = "{}", assertion = "{}", expected = "{}", actual = "{}"'.format(test_num, cmd, assertion, expected_value, output))
                print(errors[-1])
        else:
            raise Exception('invalid assertion: {} = {}'.format(assertion, expected_value))

if len(errors) > 0:
    print('Encountered {} errors:'.format(len(errors)))
    for error in errors:
        print(' > ' + error)
    exit(1)
else:
    print('Great Success!')
    exit(0)
