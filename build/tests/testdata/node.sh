#!/bin/bash

set -euo pipefail

# Test fixture, run as root in a derived container with root capabilities:
# give the node its FQDN and make it resolve locally (0.0.0.0, so slapd listens on all
# interfaces), then run the image entrypoint as the ldap user.

fqdn="$1"

hostname "${fqdn}"

cp /etc/hosts /tmp/openldap-test-hosts
echo "0.0.0.0 ${fqdn}" >> /tmp/openldap-test-hosts
mount --bind /tmp/openldap-test-hosts /etc/hosts

exec su -p -s /bin/sh ldap -c 'exec /usr/sbin/container'
