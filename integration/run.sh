#!/bin/sh
# Integration checks for yup-cut, run inside a Debian (GNU coreutils) container.
#
# parity INPUT ARGS...      — yup-cut reading stdin must produce byte-identical
#                             output to GNU `cut` reading the same stdin.
# assert WANT INPUT ARGS... — yup-cut must produce WANT exactly, used for the
#                             one documented divergence: an open-ended "-f N-"
#                             field range, which yup-cut rejects (the command
#                             package's field API selects by explicit position
#                             and cannot express an unbounded upper end).
#                             See cmd-cut COMPATIBILITY.md.
set -eu

fails=0

parity() {
	in=$1
	shift
	ours=$(printf '%s' "$in" | yup-cut "$@" 2>/dev/null || true)
	gnu=$(printf '%s' "$in" | cut "$@" 2>/dev/null || true)
	if [ "$ours" = "$gnu" ]; then
		printf 'ok    parity  cut %s\n' "$*"
	else
		printf 'FAIL  parity  cut %s\n        gnu:  %s\n        ours: %s\n' "$*" "$gnu" "$ours"
		fails=$((fails + 1))
	fi
}

assert() {
	want=$1
	in=$2
	shift 2
	got=$(printf '%s' "$in" | yup-cut "$@" 2>/dev/null || true)
	if [ "$got" = "$want" ]; then
		printf 'ok    assert  cut %s\n' "$*"
	else
		printf 'FAIL  assert  cut %s\n        want: %s\n        got:  %s\n' "$*" "$want" "$got"
		fails=$((fails + 1))
	fi
}

# Field mode (-f) with a custom delimiter (-d): single field, list, ranges.
fields='one:two:three:four:five
a:b:c:d:e'
parity "$fields" -d : -f 2
parity "$fields" -d : -f 1,3,5
parity "$fields" -d : -f 1-3
parity "$fields" -d : -f 1,3-4
# Bounded "-N" range (field 1 through N) matches GNU.
parity "$fields" -d : -f -2

# Default delimiter is TAB.
tabbed='alpha	beta	gamma'
parity "$tabbed" -f 2
parity "$tabbed" -f 1,3

# Character mode (-c): single, ranges, open-ended (-c uses a spec string).
chars='Hello, World
abcdefghij'
parity "$chars" -c 1-5
parity "$chars" -c 1,3,5
parity "$chars" -c 3-
parity "$chars" -c -4

# Byte mode (-b): same spec grammar as -c for ASCII input.
parity "$chars" -b 1-5
parity "$chars" -b 1,3,5

# --complement: invert the selected set.
parity "$fields" -d : -f 2 --complement
parity "$chars" -c 1-5 --complement

# Lines with no delimiter pass through unchanged in field mode (GNU default).
parity 'no-delimiter-here' -d : -f 1

# File operands instead of stdin.
printf '%s\n' "$fields" > /tmp/f.txt
ours_file=$(yup-cut -d : -f 1,3 /tmp/f.txt 2>/dev/null || true)
gnu_file=$(cut -d : -f 1,3 /tmp/f.txt 2>/dev/null || true)
if [ "$ours_file" = "$gnu_file" ]; then
	printf 'ok    parity  cut -d : -f 1,3 /tmp/f.txt\n'
else
	printf 'FAIL  parity  cut -d : -f 1,3 /tmp/f.txt\n        gnu:  %s\n        ours: %s\n' "$gnu_file" "$ours_file"
	fails=$((fails + 1))
fi

# Documented divergence: open-ended "-f N-" is rejected (empty stdout, exit 1),
# whereas GNU prints field N through the end of each line.
assert '' "$fields" -d : -f 2-

if [ "$fails" -ne 0 ]; then
	printf '\n%s check(s) failed\n' "$fails"
	exit 1
fi
printf '\nall checks passed\n'
