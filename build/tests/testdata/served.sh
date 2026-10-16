#!/bin/bash

set -euo pipefail

# if container log level is trace:
# print commands and their arguments as they are executed
container log level eq trace && set -x

# shellcheck disable=SC1091
source /tmp/openldap-test-lib.sh

eventually has_entry "ldap://${TEST_NODES}:3890" "uid=alice,ou=people,${TEST_SUFFIX}"
ok "upgraded data is served"
