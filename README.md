# Kamatera Docker Macine Driver

<!--
[![Go Report Card](https://goreportcard.com/badge/github.com/OriHoch/docker-machine-driver-kamatera)](https://goreportcard.com/report/github.com/OriHoch/docker-machine-driver-kamatera)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Build Status](https://secure.travis-ci.org/OriHoch/docker-machine-driver-kamatera.png)](http://travis-ci.org/OriHoch/docker-machine-driver-kamatera)
-->

**Work In Progress** Binaries are not available, you have to build from source according to the instructions below.

> This library adds the support for creating [Docker machines](https://github.com/docker/machine) hosted on the [Kamatera Cloud](https://www.kamatera.com/).

You need to create a Kamatera access token under `API` > `Keys` in the [Kamatera console](https://console.kamatera.com/keys).
and pass that to `docker-machine create` with the `--kamatera-api-client-id` and `--kamatera-api-secret` options.

<!--
## Installation

You can find sources and pre-compiled binaries [here](https://github.com/OriHoch/docker-machine-driver-kamatera/releases).

```bash
# Download the binary (this example downloads the binary for linux amd64)
$ wget https://github.com/OriHoch/docker-machine-driver-kamatera/releases/download/0.0.1/docker-machine-driver-kamatera_0.0.1_linux_amd64.tar.gz
$ tar -xvf docker-machine-driver-kamatera_0.0.1_linux_amd64.tar.gz

# Make it executable and copy the binary in a directory accessible with your $PATH
$ chmod +x docker-machine-driver-kamatera
$ cp docker-machine-driver-kamatera /usr/local/bin/
```
-->

## Usage

```bash
$ docker-machine create --driver kamatera --kamatera-api-client-id KAMATERA_API_CLIENT_ID --kamatera-api-secret KAMATERA_API_SECRET
```
   
<!--
### Dealing with kernels without aufs

If you use an image without aufs, like the one currently supplied with the
debian-9 image, you can try specifying another storage driver, such as
overlay2. Like so:

```bash
$ docker-machine create \
  --engine-storage-driver overlay2
  --driver kamatera \
  --kamatera-image debian-9 \
  --kamatera-api-token=*** \
  some-machine
```

### Using Cloud-init

```bash
$ CLOUD_INIT_USER_DATA=`cat <<EOF
#cloud-config
write_files:
  - path: /test.txt
    content: |
      Here is a line.
      Another line is here.
EOF
`

$ docker-machine create \
  --driver kamatera \
  --kamatera-api-token=QJhoRT38JfAUO037PWJ5Zt9iAABIxdxdh4gPqNkUGKIrUMd6I3cPIsfKozI513sy \
  --kamatera-user-data="${CLOUD_INIT_USER_DATA}" \
  some-machine
```

### Using a snapshot

Assuming your snapshot ID is `424242`:
```bash
$ docker-machine create \
  --driver kamatera \
  --kamatera-api-token=QJhoRT38JfAUO037PWJ5Zt9iAABIxdxdh4gPqNkUGKIrUMd6I3cPIsfKozI513sy \
  --kamatera-image-id=424242 \
  some-machine
```

## Options

- `--kamatera-api-client-id`: **required**. Your project-specific access token for the kamatera Cloud API.
- `--kamatera-api-secret`: **required**. You Kamatera API secret.
- `--kamatera-password`: **required**. Password for the new server.

#### Existing SSH keys

When you specify the `--kamatera-existing-key-path` option, the driver will attempt to copy `(specified file name)`
and `(specified file name).pub` to the machine's store path. They public key file's permissions will be set according
to your current `umask` and the private key file will have `600` permissions.

When you additionally specify the `--kamatera-existing-key-id` option, the driver will not create an SSH key using the API
but rather try to use the existing public key corresponding to the given id. Please note that during machine creation,
the driver will attempt to [get the key](https://docs.kamatera.cloud/#resources-ssh-keys-get-1) and **compare it's
fingerprint to the local public key's fingerprtint**. Keep in mind that the both the local and the remote key must be
accessible and have matching fingerprints, otherwise the machine will fail it's pre-creation checks.

Also note that the driver will attempt to delete the linked key during machine removal, unless `--kamatera-existing-key-id`
was used during creation.

#### Environment variables and default values

| CLI option                          | Environment variable              | Default                    |
| ----------------------------------- | --------------------------------- | -------------------------- |
| **`--kamatera-api-token`**           | `kamatera_API_TOKEN`               | -                          |
| `--kamatera-image`                   | `kamatera_IMAGE_IMAGE`             | `ubuntu-16.04`             |
| `--kamatera-image-id`                | `kamatera_IMAGE_IMAGE_ID`          | -                          |
| `--kamatera-server-type`             | `kamatera_TYPE`                    | `cx11`                     |
| `--kamatera-server-location`         | `kamatera_LOCATION`                | - *(let kamatera choose)*   |
| `--kamatera-existing-key-path`       | `kamatera_EXISTING_KEY_PATH`       | - *(generate new keypair)* |
| `--kamatera-existing-key-id`         | `kamatera_EXISTING_KEY_ID`         | 0 *(upload new key)*       |
| `--kamatera-user-data`               | `kamatera_USER_DATA`               | -                          |
-->

## Building from source

Use an up-to-date version of [Go](https://golang.org/dl) and [dep](https://github.com/golang/dep)

To use the driver, you can download the sources and build it locally:

```shell
# Get sources and build the binary at ~/go/bin/docker-machine-driver-kamatera
$ go get github.com/OriHoch/docker-machine-driver-kamatera

# Make the binary accessible to docker-machine
$ export GOPATH=$(go env GOPATH)
$ export GOBIN=$GOPATH/bin
$ export PATH="$PATH:$GOBIN"
$ cd $GOPATH/src/github.com/OriHoch/docker-machine-driver-kamatera
$ dep ensure
$ go build -o docker-machine-driver-kamatera
$ cp docker-machine-driver-kamatera /usr/local/bin/docker-machine-driver-kamatera
```

## Development

Fork this repository, yielding `github.com/<yourAccount>/docker-machine-driver-kamatera`.

```shell
# Get the sources of your fork and build it locally
$ go get github.com/<yourAccount>/docker-machine-driver-kamatera

# * This integrates your fork into the $GOPATH (typically pointing at ~/go)
# * Your sources are at $GOPATH/src/github.com/<yourAccount>/docker-machine-driver-kamatera
# * That folder is a local Git repository. You can pull, commit and push from there.
# * The binary will typically be at $GOPATH/bin/docker-machine-driver-kamatera
# * In the source directory $GOPATH/src/github.com/<yourAccount>/docker-machine-driver-kamatera
#   you may use go get to re-build the binary.
# * Note: when you build the driver from different repositories, e.g. from your fork
#   as well as github.com/OriHoch/docker-machine-driver-kamatera,
#   the binary files generated by these builds are all called the same
#   and will hence override each other.

# Make the binary accessible to docker-machine
$ export GOPATH=$(go env GOPATH)
$ export GOBIN=$GOPATH/bin
$ export PATH="$PATH:$GOBIN"

# Make docker-machine output help including kamatera-specific options
$ docker-machine create --driver kamatera
```
