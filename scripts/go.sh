#!/usr/bin/env bash
set -euo pipefail

repo_root=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
gopath_root=${BB_GO_GOPATH:-${TMPDIR:-/tmp}/bb-go-gopath}
import_parent=${gopath_root}/src/github.com/larslarsen
import_path=${import_parent}/bb-go
go_toolchain=${GOTOOLCHAIN:-go1.27.0}
bootstrap_go=${GO:-go}

# Automatic toolchain switching is disabled with GO111MODULE=off. Resolve the
# requested compiler before entering legacy GOPATH mode, then invoke it
# directly so `go version` and every build actually use the declared target.
toolchain_goroot=$(env GOTOOLCHAIN="${go_toolchain}" "${bootstrap_go}" env GOROOT)
go_binary=${toolchain_goroot}/bin/go

mkdir -p "${import_parent}"
if [[ -e "${import_path}" && ! -L "${import_path}" ]]; then
	echo "refusing to replace non-symlink GOPATH entry: ${import_path}" >&2
	exit 1
fi
ln -sfn "${repo_root}" "${import_path}"

cd "${repo_root}"

# GOPATH mode predates nested-module boundaries and would otherwise descend
# into modern/, then try to resolve its module dependencies from the legacy
# vendor tree. Expand ./... ourselves and keep this wrapper scoped to the
# legacy daemon. The modern module is tested from its own directory.
args=("$@")
for index in "${!args[@]}"; do
	if [[ ${args[index]} != "./..." ]]; then
		continue
	fi
	legacy_packages=()
	while IFS= read -r package; do
		case "${package}" in
			github.com/larslarsen/bb-go/modern | github.com/larslarsen/bb-go/modern/*)
				continue
				;;
		esac
		legacy_packages+=("${package}")
	done < <(env PWD="${import_path}" GO111MODULE=off GOPATH="${gopath_root}" "${go_binary}" list -e ./...)
	args=("${args[@]:0:index}" "${legacy_packages[@]}" "${args[@]:index+1}")
	break
done

env \
	PWD="${import_path}" \
	GO111MODULE=off \
	GOPATH="${gopath_root}" \
	"${go_binary}" "${args[@]}"
