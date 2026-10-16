#!/bin/bash

set -euo pipefail

# if container log level is trace:
# print commands and their arguments as they are executed
container log level eq trace && set -x

CONFIG=false
DATA=false
FORCE=false
RESTART=false
RESTORED=false
FILE_PREFIX=""

# existing config and data are moved to this hidden directory, in the same volume, until
# the restore succeeds
PREVIOUS_DIR_NAME=".restore-previous"
STASHED_DIRS=()

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

# move the content of a directory to its previous directory
stash() {
    local dir="$1"
    local previous_dir="${dir}/${PREVIOUS_DIR_NAME}"

    container log info "Moving existing data in ${dir} to ${previous_dir} ..."
    mkdir "${previous_dir}"
    STASHED_DIRS+=("${dir}")

    find "${dir}" -mindepth 1 -maxdepth 1 ! -name "lost+found" ! -name ".*" \
        -exec mv -v {} "${previous_dir}/" \; 2>&1 | container log debug
}

# replace the content of a directory by its previous directory content
unstash() {
    local dir="$1"
    local previous_dir="${dir}/${PREVIOUS_DIR_NAME}"

    container log warning "Restoring previous data in ${dir} ..."

    find "${dir}" -mindepth 1 -maxdepth 1 ! -name "lost+found" ! -name ".*" -exec rm -fr {} +
    find "${previous_dir}" -mindepth 1 -maxdepth 1 -exec mv -v {} "${dir}/" \; 2>&1 | container log debug
    rmdir "${previous_dir}"
}

finish() {
    local dir

    if [ "${RESTORED}" = true ]; then
        for dir in "${STASHED_DIRS[@]}"; do
            rm -fr "${dir:?}/${PREVIOUS_DIR_NAME}"
        done
    elif [ "${#STASHED_DIRS[@]}" -gt 0 ]; then
        container log error "Restore failed, rolling back to the previous data ..."
        for dir in "${STASHED_DIRS[@]}"; do
            unstash "${dir}"
        done
    fi

    if [ "${RESTART}" = true ]; then
        container processes start openldap
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

for dir in "${OPENLDAP_CONF_DIR}" "${OPENLDAP_DATA_DIR}"; do
    if [ -e "${dir}/${PREVIOUS_DIR_NAME}" ]; then
        container log fatal "${dir}/${PREVIOUS_DIR_NAME} exists: a previous restore was interrupted. Check the data, then move its content back to ${dir} or delete it."
    fi
done

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

trap finish EXIT

if [ "${CONFIG}" = true ]; then
    stash "${OPENLDAP_CONF_DIR}"

    container log info "Restoring OpenLDAP config ..."
    restore_database "${CONFIG_BACKUP_FILE}" 0
fi

if [ "${DATA}" = true ]; then
    stash "${OPENLDAP_DATA_DIR}"

    container log info "Restoring OpenLDAP data ..."
    restore_database "${DATA_BACKUP_FILE}" 1
fi

RESTORED=true
