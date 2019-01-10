FROM ubuntu

RUN apt-get update && apt-get install -y curl python3 apt-transport-https ca-certificates software-properties-common &&\
    curl -L https://github.com/docker/machine/releases/download/v0.16.0/docker-machine-$(uname -s)-$(uname -m) >/tmp/docker-machine &&\
    install /tmp/docker-machine /usr/local/bin/docker-machine &&\
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | apt-key add - &&\
    apt-key fingerprint 0EBFCD88 &&\
    add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" &&\
    apt-get update && apt-get install -y docker-ce
COPY docker-machine-driver-kamatera /usr/local/bin/docker-machine-driver-kamatera
COPY *.py *.sh /
ENTRYPOINT ["/entrypoint.sh"]
