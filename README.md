# Openwrt Terraform Provider

Openwrt Provider for Terraform (or OpenTofu).

## Prerequisites

- [Terraform](https://terraform.io) or [OpenTofu](https://opentofu.org/)
- [An OpenWRT Router](https://openwrt.org)
- `luasocket` `luci-mod-rpc` `luci-lib-ipkg` and `luci-compat` packages installed in the openwrt router

## Installation

This provider is published in the [Terraform Registry](https://registry.terraform.io/providers/foxboron/openwrt).

### Quick Example

Add the following to your Terraform configuration:

```hcl
terraform {
  required_providers {
    openwrt = {
      source = "foxboron/openwrt"
    }
  }
}
```

## Documentation

Full documentation can be found in the [`docs`](docs) directory.

## Development

### Prerequisites

- [Golang](https://golang.org/doc/install)
- [Make](https://www.gnu.org/software/make/)
- [Golangci-lint](https://golangci-lint.run/)
- [GoReleaser](https://goreleaser.com/)

### Setup

1. Checkout the repository `git clone ...`
2. Compile from sources to a development binary:

```shell
cd terraform-provider-openwrt
make build
```

3. Configure Terraform (`~/.terraformrc`) to use the development binary provider:

```shell
$ cat ~/.terraformrc
provider_installation {
  dev_overrides {
    "foxboron/openwrt" = "/<PATH_WHERE_YOU_EXECUTED_GIT_CLONE>/terraform-provider-openwrt"
  }
  direct {}
}
```

### Test routers

`testenv/openwrt-containers.sh` starts throwaway OpenWrt containers (23.05, 24.10 and 25.12 by default) with the LuCI JSON-RPC API, each on `127.0.0.1:180<major>`. It needs `podman` or `docker` and an x86_64 host.

```shell
testenv/openwrt-containers.sh up
testenv/openwrt-containers.sh status
testenv/openwrt-containers.sh down
```

Point the provider at one of them:

```hcl
provider "openwrt" {
  remote   = "http://127.0.0.1:18024"
  user     = "root"
  password = "openwrt-test"
}
```
