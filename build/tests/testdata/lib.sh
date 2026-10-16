#!/bin/bash
# shellcheck disable=SC2034

# Helpers sourced by the test scripts.

ADMIN_DN="cn=admin,${TEST_SUFFIX}"
CONFIG_ADMIN_DN="cn=admin,cn=config"

OUTPUT_FILE="$(mktemp)"

ok() {
    echo "ok: $*"
}

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

# expect_rc <rc> <command...>: run command and check its exit code, output is kept in
# OUTPUT_FILE
expect_rc() {
    local want="$1"
    shift

    local rc=0
    "$@" > "${OUTPUT_FILE}" 2>&1 || rc=$?

    if [ "${rc}" != "${want}" ]; then
        cat "${OUTPUT_FILE}" >&2
        fail "expected exit code ${want}, got ${rc}: $*"
    fi
}

# expect_output <pattern>: check the last command output matches an extended regex
expect_output() {
    if ! grep -qE "$1" "${OUTPUT_FILE}"; then
        cat "${OUTPUT_FILE}" >&2
        fail "output does not match '$1'"
    fi
}

# expect_no_output <pattern>: check the last command output does not match an extended regex
expect_no_output() {
    if grep -qE "$1" "${OUTPUT_FILE}"; then
        cat "${OUTPUT_FILE}" >&2
        fail "output matches '$1'"
    fi
}

# eventually <command...>: retry command for up to 60 seconds
eventually() {
    local i
    for i in $(seq 1 60); do
        if "$@" > "${OUTPUT_FILE}" 2>&1; then
            return 0
        fi
        sleep 1
    done

    cat "${OUTPUT_FILE}" >&2
    fail "timeout after ${i} attempts: $*"
}

# bind_search <uri> <bind dn> <ldapsearch args...>: simple bind with the test password
bind_search() {
    local uri="$1"
    local dn="$2"
    shift 2
    ldapsearch -LLL -o ldif-wrap=no -x -H "${uri}" -D "${dn}" -w "${TEST_PASSWORD}" "$@"
}

# bind_modify <uri> <bind dn> <ldapmodify args...>: simple bind with the test password, LDIF on stdin
bind_modify() {
    local uri="$1"
    local dn="$2"
    shift 2
    ldapmodify -x -H "${uri}" -D "${dn}" -w "${TEST_PASSWORD}" "$@"
}

# has_entry <uri> <dn>: check an entry exists, as admin
has_entry() {
    bind_search "$1" "${ADMIN_DN}" -b "$2" -s base dn | grep -qi "^dn: $2$"
}

# expect_fail <command...>: run command and check it fails, output is kept in OUTPUT_FILE
expect_fail() {
    if "$@" > "${OUTPUT_FILE}" 2>&1; then
        cat "${OUTPUT_FILE}" >&2
        fail "expected failure: $*"
    fi
}
