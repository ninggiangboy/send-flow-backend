#!/bin/sh
set -eu

min_port="${LOCAL_RANDOM_PORT_MIN:-20000}"
max_port="${LOCAL_RANDOM_PORT_MAX:-60999}"
used=" $* "

case "$min_port:$max_port" in
	*[!0-9:]* | :* | *:)
		echo "LOCAL_RANDOM_PORT_MIN and LOCAL_RANDOM_PORT_MAX must be numeric" >&2
		exit 2
		;;
esac

if [ "$min_port" -le 0 ] || [ "$max_port" -lt "$min_port" ] || [ "$max_port" -gt 65535 ]; then
	echo "invalid local random port range: ${min_port}-${max_port}" >&2
	exit 2
fi

i=0
while [ "$i" -lt 200 ]; do
	if command -v python3 >/dev/null 2>&1; then
		port="$(python3 -c "import random; print(random.randint($min_port, $max_port))")"
	else
		port="$(awk -v min="$min_port" -v max="$max_port" -v seed="$$:$i:$(date +%s)" 'BEGIN { srand(seed); print int(min + rand() * (max - min + 1)) }')"
	fi

	case "$used" in
		*" $port "*)
			i=$((i + 1))
			continue
			;;
	esac

	if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
		i=$((i + 1))
		continue
	fi

	if command -v python3 >/dev/null 2>&1; then
		if python3 - "$port" <<'PY' >/dev/null 2>&1
import socket
import sys

port = int(sys.argv[1])
sockets = []
try:
    for host in ("0.0.0.0", "::"):
        family = socket.AF_INET6 if ":" in host else socket.AF_INET
        sock = socket.socket(family, socket.SOCK_STREAM)
        sock.bind((host, port))
        sockets.append(sock)
except OSError:
    sys.exit(1)
finally:
    for sock in sockets:
        sock.close()
PY
		then
			echo "$port"
			exit 0
		fi
	else
		echo "$port"
		exit 0
	fi

	i=$((i + 1))
done

echo "could not find an available local port in ${min_port}-${max_port}" >&2
exit 1
