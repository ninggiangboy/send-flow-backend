#!/bin/sh

label="$1"
color="$2"
use_color="$3"
reset=$(printf '\033[0m')
bold=$(printf '\033[1m')
dim=$(printf '\033[2m')
red=$(printf '\033[1;31m')
yellow=$(printf '\033[1;33m')
green=$(printf '\033[1;32m')
blue=$(printf '\033[1;34m')

jq --unbuffered -Rr '
	. as $raw
	| (
	def fmt:
		if . == null then empty
		elif type == "string" then .
		elif type == "array" or type == "object" then @json
		else tostring
		end;
	def omit:
		del(.time, .level, .msg, .app, .env, .service, .instance_id, .instance_addr);
	try (
		fromjson as $o
		| if ($o | type) != "object" then
			$raw
		else
			[
				($o.time // "-"),
				($o.level // "INFO"),
				($o.msg // "")
			] | join(" ")
			+
			(
				$o
				| omit
				| to_entries
				| map("\(.key)=\(.value | fmt)")
				| if length > 0 then " " + join(" ") else "" end
			)
		end
	) catch $raw
	)
' 2>/dev/null | awk \
	-v label="$label" \
	-v color="$color" \
	-v reset="$reset" \
	-v use_color="$use_color" \
	-v bold="$bold" \
	-v dim="$dim" \
	-v red="$red" \
	-v yellow="$yellow" \
	-v green="$green" \
	-v blue="$blue" '
	function paint_level(level) {
		if (level == "ERROR") return red level reset
		if (level == "WARN") return yellow level reset
		if (level == "INFO") return green level reset
		if (level == "DEBUG") return blue level reset
		return level
	}

	function short_ts(ts, parts, time_part) {
		split(ts, parts, "T")
		if (length(parts) < 2) {
			return ts
		}

		time_part = parts[2]
		sub(/[+-][0-9]{2}:[0-9]{2}$/, "", time_part)
		sub(/Z$/, "", time_part)
		return time_part
	}

	function format_message(line, parts, ts, level, msg, rest, idx, out) {
		split(line, parts, " ")
		if (length(parts) < 3) {
			return line
		}

		ts = parts[1]
		level = parts[2]
		msg = ""
		rest = ""

		for (idx = 3; idx <= length(parts); idx++) {
			if (parts[idx] ~ /^[A-Za-z0-9_.-]+=/) {
				break
			}
			if (msg != "") {
				msg = msg " "
			}
			msg = msg parts[idx]
		}

		for (; idx <= length(parts); idx++) {
			if (rest != "") {
				rest = rest " "
			}
			rest = rest parts[idx]
		}

		out = short_ts(ts) " " level
		if (msg != "") {
			out = out " " msg
		}
		if (rest != "") {
			out = out " " rest
		}

		if (use_color != "1") {
			return out
		}

		out = dim short_ts(ts) reset " " paint_level(level)
		if (msg != "") {
			out = out " " bold msg reset
		}
		if (rest != "") {
			out = out " " rest
		}
		return out
	}

	{
		formatted = format_message($0)
		if (use_color == "1") {
			printf "%s[%s]%s %s\n", color, label, reset, formatted
		} else {
			printf "[%s] %s\n", label, formatted
		}
		fflush()
	}
'
