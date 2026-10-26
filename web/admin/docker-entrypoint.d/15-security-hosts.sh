#!/bin/sh
set -eu
# This file is generated from trusted deployment configuration, never a request.
{
  printf 'map $host $security_host_allowed {\n  default 0;\n'
  if [ -z "${SECURITY_ALLOWED_HOSTS:-}" ]; then
    printf '  localhost 1;\n  "::1" 1;\n  "~^([0-9]{1,3}\\.){3}[0-9]{1,3}$" 1;\n'
  else
    for security_host in $(printf '%s' "$SECURITY_ALLOWED_HOSTS" | tr ',' ' '); do
      case "$security_host" in *[!a-zA-Z0-9.:-]*|'') echo 'Invalid SECURITY_ALLOWED_HOSTS' >&2; exit 1;; esac
      printf '  "%s" 1;\n' "$security_host"
    done
  fi
  printf '}\n'
} > /etc/nginx/conf.d/security-hosts.conf
