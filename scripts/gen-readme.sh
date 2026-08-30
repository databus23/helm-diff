#!/usr/bin/env bash
# Regenerates the cobra flag tables in README.md from the --help output of the
# diff binary, so the documented flags cannot drift from the actual flags.
#
# Usage: scripts/gen-readme.sh [path-to-diff-binary]   (default: bin/diff)
#
# The README contains one flag table per command, in the order listed in
# COMMANDS below. Only the contiguous block of indented flag rows that follows
# each "Flags:" line is rewritten; any surrounding prose is left untouched.
# The script fails if the number of "Flags:" tables found does not match the
# number of commands, so a lost or accidentally added table cannot slip through.
set -euo pipefail

BIN="${1:-bin/diff}"

if [ ! -x "${BIN}" ]; then
	echo "diff binary not found or not executable at ${BIN}. Run 'make build' first." >&2
	exit 1
fi

# The README documents the flags of these commands, in this order.
# The first table belongs to the (deprecated) root command, which carries the
# same flag set as "upgrade".
COMMANDS=("" local upgrade release revision rollback)

WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

# extract_flags prints the indented flag rows of the "Flags:" section of the
# given subcommand's --help output. An empty subcommand selects the root
# command. HELM_NAMESPACE and HELM_DIFF_STORAGE_NAMESPACE are unset so that
# no environment default leaks into the rendered flag defaults.
extract_flags() {
	local subcmd="$1"
	local args=()
	if [ -n "${subcmd}" ]; then
		args+=("${subcmd}")
	fi
	env -u HELM_NAMESPACE -u HELM_DIFF_STORAGE_NAMESPACE "${BIN}" ${args[@]+"${args[@]}"} --help | awk '
		/^Flags:$/ { in_flags = 1; next }
		in_flags && /^  / { print; next }
		in_flags { exit }
	'
}

i=0
for subcmd in "${COMMANDS[@]}"; do
	if ! extract_flags "${subcmd}" > "${WORKDIR}/flags_${i}.txt" || [ ! -s "${WORKDIR}/flags_${i}.txt" ]; then
		echo "failed to extract flag table for command '${subcmd:-<root>}'" >&2
		exit 1
	fi
	i=$((i + 1))
done

README="${README:-README.md}"

awk -v workdir="${WORKDIR}" -v n="${#COMMANDS[@]}" -v readme="${README}" '
	function load_rows(i, line) {
		rows[i] = ""
		while ((getline line < (workdir "/flags_" i ".txt")) > 0)
			rows[i] = rows[i] line "\n"
		close(workdir "/flags_" i ".txt")
	}
	BEGIN {
		for (i = 0; i < n; i++) load_rows(i)
	}
	/^Flags:$/ { total++ }
	/^Flags:$/ && idx < n {
		print
		idx++
		printf "%s", rows[idx - 1]
		in_rows = 1
		next
	}
	in_rows && /^  / { next }
	{ in_rows = 0; print }
	END {
		if (total != n || idx != n) {
			printf "expected %d flag tables in %s, found %d\n", n, readme, total > "/dev/stderr"
			exit 1
		}
	}
' "${README}" > "${WORKDIR}/README.new"

mv "${WORKDIR}/README.new" "${README}"
echo "Regenerated ${#COMMANDS[@]} flag tables in ${README}"
