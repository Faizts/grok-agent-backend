#!/bin/bash
set -e

mkdir -p /var/log/supervisor /workspace/.browser
# Chromium singleton markers refer to the old container hostname after recreation.
rm -f /workspace/.browser/SingletonLock /workspace/.browser/SingletonSocket /workspace/.browser/SingletonCookie

# Wait a moment then start supervisor
exec /usr/bin/supervisord -n -c /etc/supervisor/conf.d/supervisord.conf
