#!/usr/bin/env bash

( [ -z "${KAMATERA_API_CLIENT_ID}" ] || [ -z "${KAMATERA_API_SECRET}" ] ) && exit 1

MACHINE="docker-machine-`date +%Y%m%d%H%M%S`"
echo MACHINE=$MACHINE
docker-machine create -d kamatera $MACHINE &&\
docker-machine restart $MACHINE &&\
docker-machine stop $MACHINE &&\
docker-machine start $MACHINE &&\
docker-machine kill $MACHINE &&\
docker-machine restart $MACHINE &&\
[ "$(docker-machine ssh $MACHINE hostname)" == "${MACHINE}" ]
[ "$?" != "0" && echo ERROR! docker-machine test failed

docker-machine rm -f $MACHINE

exit 0
