#!/bin/sh
set -eu

mode="${DAUNRODO_FAKE_MODE:-success}"

for arg in "$@"; do
	if [ "$arg" = "-F" ]; then
		printf '%s\n' '{"duration":120,"formats":[{"format_id":"18","filesize":5242880}]}'
		exit 0
	fi
done

if [ "$mode" = "fail" ]; then
	printf '%s\n' 'simulated process exit' >&2
	exit 2
fi

out="${DAUNRODO_FAKE_OUTPUT_FILE:-/tmp/daunrodo-fake.mp4}"
mkdir -p "$(dirname "$out")"
printf '%s' 'fake-media-bytes' > "$out"

for p in 10 35 70 100; do
	printf '%s\n' "[download] ${p}.0% of 10.00MiB at 1.00MiB/s ETA 00:01"
	if [ "$mode" = "slow" ]; then
		sleep 0.3
	fi
done

printf '%s\n' '{"id":"vid-123","title":"Fake title","extractor":"youtube","webpage_url":"https://example.com/watch?v=vid-123","uploader":"integration-test","description":"fake result","thumbnail":"https://example.com/thumb.jpg","view_count":42}'
printf '%s\n' "$out"
