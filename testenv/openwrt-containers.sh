#!/usr/bin/env bash
# Copyright https://github.com/Foxboron/terraform-provider-openwrt/graphs/contributors 2026
# SPDX-License-Identifier: MPL-2.0

# Start, check and stop throwaway OpenWrt containers with the LuCI JSON-RPC
# API, one per OpenWrt release, to run the provider against.
#
# Usage: testenv/openwrt-containers.sh up|status|down
#
# Environment:
#   OPENWRT_VERSIONS       releases to run (default: "23.05.6 24.10.8 25.12.6")
#   OPENWRT_ENGINE         podman or docker (default: podman if installed)
#   OPENWRT_TEST_PASSWORD  root password set in the containers (default: openwrt-test)
#
# Each release listens on 127.0.0.1:180<major>, e.g. 23.05.6 on :18023.

set -euo pipefail

versions=${OPENWRT_VERSIONS:-"23.05.6 24.10.8 25.12.6"}
password=${OPENWRT_TEST_PASSWORD:-openwrt-test}
engine=${OPENWRT_ENGINE:-$(command -v podman >/dev/null && echo podman || echo docker)}
context=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

name() { echo "openwrt-rpc-$1"; }
port() { echo "180${1%%.*}"; }

login() {
	curl --silent --fail --max-time 5 \
		--data "{\"id\":1,\"method\":\"login\",\"params\":[\"root\",\"$2\"]}" \
		"http://127.0.0.1:$(port "$1")/cgi-bin/luci/rpc/auth" |
		grep --quiet '"result":"[0-9a-f]'
}

up() {
	local v
	for v in $versions; do
		echo "==> OpenWrt $v"
		"$engine" build --quiet --build-arg "OPENWRT_VERSION=$v" \
			--tag "localhost/openwrt-rpc:$v" "$context" >/dev/null
		"$engine" rm --force "$(name "$v")" >/dev/null 2>&1 || true
		"$engine" run --detach --stop-timeout 1 --name "$(name "$v")" \
			--publish "127.0.0.1:$(port "$v"):80" "localhost/openwrt-rpc:$v" >/dev/null

		# The fresh rootfs has an empty root password; uhttpd needs a moment.
		local i
		for i in $(seq 60); do
			login "$v" "" && break
			[ "$i" = 60 ] && { echo "LuCI RPC did not come up for $v" >&2; exit 1; }
			sleep 1
		done
		"$engine" exec "$(name "$v")" sh -c \
			"printf '%s\n%s\n' '$password' '$password' | passwd root >/dev/null 2>&1"
		login "$v" "$password" || { echo "login with test password failed for $v" >&2; exit 1; }
		echo "    remote http://127.0.0.1:$(port "$v")"
	done
	echo
	echo "Provider settings: user = \"root\", password = \"$password\""
}

status() {
	local v
	for v in $versions; do
		if login "$v" "$password"; then
			echo "$v  http://127.0.0.1:$(port "$v")  up"
		else
			echo "$v  http://127.0.0.1:$(port "$v")  down"
		fi
	done
}

down() {
	local v
	for v in $versions; do
		"$engine" rm --force "$(name "$v")" >/dev/null 2>&1 && echo "removed $(name "$v")" || true
	done
}

case "${1:-}" in
up | status | down) "$1" ;;
*)
	echo "usage: $0 up|status|down" >&2
	exit 2
	;;
esac
