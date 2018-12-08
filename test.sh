#!/usr/bin/env bash

( [ -z "${KAMATERA_API_CLIENT_ID}" ] || [ -z "${KAMATERA_API_SECRET}" ] ) && exit 1

MACHINES="docker-machine-1-`date +%Y%m%d%H%M%S` docker-machine-2-`date +%Y%m%d%H%M%S` docker-machine-3-`date +%Y%m%d%H%M%S`"

wait_state() {
    echo Waiting for machines to be $1
    while true; do
        sleep 1
        STATES=""
        for MACHINE in $MACHINES; do
            [ "$(docker-machine status $MACHINE 2>/dev/null)" == "$1" ] && STATES="${STATES}-"
        done
        [ "${STATES}" == "---" ] && break
    done
}

echo Creating machines
for MACHINE in $MACHINES; do docker-machine create -d kamatera $MACHINE; done
wait_state Running

echo Restarting machines
for MACHINE in $MACHINES; do docker-machine restart $MACHINE; done
wait_state Running

echo Stopping machines
for MACHINE in $MACHINES; do docker-machine stop $MACHINE; done
wait_state Stopped

echo Starting machines
for MACHINE in $MACHINES; do docker-machine start $MACHINE; done
wait_state Running

echo Killing machines
for MACHINE in $MACHINES; do docker-machine kill $MACHINE; done
wait_state Stopped

echo Restarting machines
for MACHINE in $MACHINES; do docker-machine restart $MACHINE; done
wait_state Running

for MACHINE in $MACHINES; do
    [ "$(docker-machine ssh $MACHINE hostname)" != "${MACHINE}" ] && exit 1
done

for MACHINE in $MACHINES; do docker-machine rm -f $MACHINE; done

exit 0