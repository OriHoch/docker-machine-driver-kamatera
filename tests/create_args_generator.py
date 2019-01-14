#!/usr/bin/env python3.6
import os
import subprocess
import json
import sys
import random


KAMATERA_OPTS_TO_DRIVER_ARGS = {
    'datacenters': '--kamatera-datacenter',
    'cpu': '--kamatera-cpu',
    'ram': '--kamatera-ram',
    'disk': '--kamatera-disk-size',
    'diskImages': None,
    'traffic': None,
    'networks': None,
    'billing': None
}


def create_args_generator():
    if os.path.exists('kamatera_server_options.json'):
        print('Loading from local file kamatera_server_options.json, delete to fetch fresh options')
        returncode, output = subprocess.getstatusoutput('cat kamatera_server_options.json')
    else:
        print('Fetching Kamatera server options')
        returncode, output = subprocess.getstatusoutput(
                'curl -s -H "AuthClientId: ${KAMATERA_API_CLIENT_ID}" -H "AuthSecret: ${KAMATERA_API_SECRET}" '
                '"https://console.kamatera.com/service/server" | tee kamatera_server_options.json')
    assert returncode == 0, output
    while True:
        create_args = []
        for opt, values in json.loads(output).items():
            arg = KAMATERA_OPTS_TO_DRIVER_ARGS[opt]
            if arg is None: continue
            if opt == 'datacenters':
                values = list(values.keys())
                values = [v for v in values if v not in ['DEV']]
            elif opt == 'cpu':
                values = [v for v in values if int(v.strip('BD')) < 12]
            elif opt == 'ram':
                values = [v for v in values if int(v) < 8192]
            value = random.choice(values)
            create_args += [arg, str(value)]
        yield create_args


if __name__ == '__main__':
    if not os.environ.get('KAMATERA_API_CLIENT_ID') or not os.environ.get('KAMATERA_API_SECRET'):
        print('Missing required env vars: KAMATERA_API_CLIENT_ID KAMATERA_API_SECRET')
        exit(1)
    assert len(sys.argv) == 2, 'usage: create_args_generator.py <HOW_MANY_TO_GENERATE>'
    how_many = int(sys.argv[1])
    print('HOW_MANY_TO_GENERATE =', how_many)
    g = create_args_generator()
    for _ in range(how_many):
        print(next(g))
