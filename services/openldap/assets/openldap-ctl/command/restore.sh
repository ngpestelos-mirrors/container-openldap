#!/bin/bash

set -euo pipefail

# if container log level is trace:
# print commands and their arguments as they are executed
container log level eq trace && set -x

CONFIG=false
DATA=false
FORCE=false
RESTART=false
FILE_PREFIX=""

usage() {
    echo "Usage: openldap-ctl restore file-prefix [--config] [--data] [--force]"
}

check_backup_file() {
    local backup_file="$1"

    if [ ! -f "${backup_file}" ]; then
        container log fatal "Backup file not found: ${backup_file}"
    fi

    if ! gzip -t "${backup_file}" 2>/dev/null; then
        container log fatal "Backup file is corrupted: ${backup_file}"
    fi
}

restore_database() {
    local backup_file="$1"
    local db_number="$2"

    if [ ! -f "${backup_file}" ]; then
        container log fatal "Backup file not found: ${backup_file}"
    fi

    gzip -dc "${backup_file}" | slapadd -F "${OPENLDAP_CONF_DIR}" -n "${db_number}" 2>&1 | container log debug

    if [ "${db_number}" -gt 0 ]; then
        slapindex -F "${OPENLDAP_CONF_DIR}" -n "${db_number}" 2>&1 | container log debug
    fi
}

while [ $# -gt 0 ]; do
    case "$1" in
        --config)
            CONFIG=true
            shift
            ;;
        --data)
            DATA=true
            shift
            ;;
        --force)
            FORCE=true
            shift
            ;;
        --*)
            usage
            container log fatal "Unknown option: $1"
            ;;
        *)
            if [ -z "${FILE_PREFIX}" ]; then
                FILE_PREFIX="$1"
            else
                usage
                container log fatal "Too many arguments"
            fi
            shift
            ;;
    esac
done

if [ -z "${FILE_PREFIX}" ]; then
    usage
    container log fatal "Missing file-prefix"
fi

if [ "${CONFIG}" = false ] && [ "${DATA}" = false ]; then
    CONFIG=true
    DATA=true
fi

CONFIG_BACKUP_FILE="${OPENLDAP_BACKUP_DIR}/${FILE_PREFIX}${OPENLDAP_CTL_BACKUP_CONFIG_FILE_SUFFIX}"
DATA_BACKUP_FILE="${OPENLDAP_BACKUP_DIR}/${FILE_PREFIX}${OPENLDAP_CTL_BACKUP_DATA_FILE_SUFFIX}"

if [ "${CONFIG}" = true ]; then
    check_backup_file "${CONFIG_BACKUP_FILE}"
fi

if [ "${DATA}" = true ]; then
    check_backup_file "${DATA_BACKUP_FILE}"
fi

if [ -e "/run/container/processes/openldap.pid" ]; then

    if [ "${FORCE}" != true ]; then
        container log fatal "OpenLDAP is currently running. Restore cannot proceed. Use --force to stop the service."
    fi

    container processes stop openldap

    RESTART=true

fi

if [ -n "$(find "${OPENLDAP_CONF_DIR}" -mindepth 1 -maxdepth 1 ! -name "lost+found" ! -name ".*")" ] || \
   [ -n "$(find "${OPENLDAP_DATA_DIR}" -mindepth 1 -maxdepth 1 ! -name "lost+found" ! -name ".*")" ]; then

    if [ "${FORCE}" != true ]; then
        container log fatal "Existing data detected in ${OPENLDAP_CONF_DIR} or ${OPENLDAP_DATA_DIR}. Restore cannot proceed. Use --force to delete existing data, but make sure to back it up first."
    fi

fi

if [ "${CONFIG}" = true ]; then

    if [ -n "$(find "${OPENLDAP_CONF_DIR}" -mindepth 1 -maxdepth 1 ! -name "lost+found" ! -name ".*")" ]; then
        container log warning "Deleting data in ${OPENLDAP_CONF_DIR} ..."
        rm -fr "${OPENLDAP_CONF_DIR:?}"/*
    fi

    container log info "Restoring OpenLDAP config ..."
    restore_database "${CONFIG_BACKUP_FILE}" 0
fi

if [ "${DATA}" = true ]; then

    if [ -n "$(find "${OPENLDAP_DATA_DIR}" -mindepth 1 -maxdepth 1 ! -name "lost+found" ! -name ".*")" ]; then
        container log warning "Deleting data in ${OPENLDAP_DATA_DIR} ..."
        rm -fr "${OPENLDAP_DATA_DIR:?}"/*
    fi

    container log info "Restoring OpenLDAP data ..."
    restore_database "${DATA_BACKUP_FILE}" 1
fi

if [ "${RESTART}" = true ]; then
    container processes start openldap
fi
