#!/bin/bash
set -e

mkdir -p /var/log/supervisor /workspace

# Wait a moment then start supervisor
exec /usr/bin/supervisord -n -c /etc/supervisor/conf.d/supervisord.conf
