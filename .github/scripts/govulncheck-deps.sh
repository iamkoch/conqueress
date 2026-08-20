#!/usr/bin/env bash
#
# Runs govulncheck over every module and fails only on a vulnerability that is
# reachable from our code in one of our dependencies.
#
# Standard library findings are reported but do not fail the run. They track
# the Go toolchain the runner happens to have, new ones appear whenever that
# toolchain is a patch behind, and no change to this repository clears them.
# Update the toolchain for those.

set -euo pipefail

modules=("$@")
if [ ${#modules[@]} -eq 0 ]; then
	modules=(. firestore mongo)
fi

called='.[] | select(.finding) | .finding | select(.trace[0].function != null)'
deps_filter="[${called} | select(.trace[0].module != \"stdlib\") | .osv] | unique"
std_filter="[${called} | select(.trace[0].module == \"stdlib\") | .osv] | unique"

status=0

for module in "${modules[@]}"; do
	echo "== ${module} =="

	report=$(mktemp)
	trap 'rm -f "${report}"' EXIT

	(cd "${module}" && GOWORK=off govulncheck -format json ./...) >"${report}"

	deps=$(jq -s --raw-output "${deps_filter} | join(\", \")" "${report}")
	std=$(jq -s --raw-output "${std_filter} | length" "${report}")

	if [ -n "${deps}" ]; then
		echo "  reachable in dependencies: ${deps}"
		status=1
	else
		echo "  no reachable vulnerability in dependencies"
	fi

	echo "  reachable in the Go standard library: ${std} (update the toolchain to clear these)"

	rm -f "${report}"
	trap - EXIT
done

exit "${status}"
