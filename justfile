setup:
	k3d cluster list | grep appthrust-tokenaut > /dev/null 2>&1 || k3d cluster create --config cluster.yaml

teardown:
	k3d cluster delete --config cluster.yaml

download-crds:
	#!/usr/bin/env bash
	[ -f crds.yaml ] && rm crds.yaml
	kubectl get crd -o custom-columns=NAME:.metadata.name | grep -E "crossplane.io|suin.jp|github.upbound.io" | while read -r crd; do
		kubectl get crd "$crd" -o yaml >> "crds.yaml"
		echo "---" >> "crds.yaml"
	done

demo-all:
	#!/usr/bin/env zsh
	set -euo pipefail
	for file in **/demo.zsh; do
		pushd $(dirname $file)
		gum style --foreground 288 --border rounded --border-foreground 288 --padding "0 1" "Starting $file"
		./demo.zsh
		popd
	done
