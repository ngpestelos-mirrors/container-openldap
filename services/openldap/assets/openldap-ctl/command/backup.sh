#!/bin/bash

set -euo pipefail

# if container log level is trace:
# print commands and their arguments as they are executed
container log level eq trace && set -x

CONFIG=false
DATA=false
CLEAN_DAYS=""
CLEAN_PREFIX=""
FILE_PREFIX=""

usage() {
    echo "Usage: openldap-ctl backup file-prefix [--config] [--data] [--clean days|--clean=days] [--clean-prefix prefix|--clean-prefix=prefix]"
}

# remove backup files older than $1 days, only those starting with $2 if set
clean() {
    find "${OPENLDAP_BACKUP_DIR}" -mindepth 1 -maxdepth 1 -type f -mtime "+$1" \
        \( -name "$2*${OPENLDAP_CTL_BACKUP_CONFIG_FILE_SUFFIX}" -o -name "$2*${OPENLDAP_CTL_BACKUP_DATA_FILE_SUFFIX}" \) \
        -exec rm -fv {} + | container log info
}

backup_database() {
    local filename="$1"
    local db_number="$2"
    local tmp_file

    umask 077

    tmp_file="$(mktemp "${OPENLDAP_BACKUP_DIR}/.tmp.XXXXXX")"
    trap 'rm -f "${tmp_file}"' RETURN

    if ! slapcat -F "${OPENLDAP_CONF_DIR}" -n "${db_number}" | gzip > "${tmp_file}"; then
        container log fatal "Failed to backup database ${db_number} to ${filename}"
    fi

    mv -v "${tmp_file}" "${OPENLDAP_BACKUP_DIR}/${filename}" 2>&1 | container log debug

    trap - RETURN
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
      --clean)
          if [ $# -lt 2 ]; then
              usage
              container log fatal "Missing value for --clean"
          fi
          CLEAN_DAYS="$2"
          shift 2
          ;;
      --clean=*)
          CLEAN_DAYS="${1#*=}"
          shift
          ;;
      --clean-prefix)
          if [ $# -lt 2 ]; then
              usage
              container log fatal "Missing value for --clean-prefix"
          fi
          CLEAN_PREFIX="$2"
          shift 2
          ;;
      --clean-prefix=*)
          CLEAN_PREFIX="${1#*=}"
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

if [ -n "${CLEAN_DAYS}" ] && [[ ! "${CLEAN_DAYS}" =~ ^[0-9]+$ ]]; then
    usage
    container log fatal "--clean must be a non-negative number of days"
fi

if [ -n "${CLEAN_PREFIX}" ] && [ -z "${CLEAN_DAYS}" ]; then
    usage
    container log fatal "--clean-prefix requires --clean"
fi

if [[ "${CLEAN_PREFIX}" == *[\*\?\[/]* ]]; then
    usage
    container log fatal "--clean-prefix must not contain *, ?, [ or /"
fi

if [ "${CONFIG}" = false ] && [ "${DATA}" = false ]; then
    CONFIG=true
    DATA=true
fi

if [ "${CONFIG}" = true ]; then
    container log info "Backing up OpenLDAP config ..."
    backup_database "${FILE_PREFIX}${OPENLDAP_CTL_BACKUP_CONFIG_FILE_SUFFIX}" 0
fi

if [ "${DATA}" = true ]; then
    container log info "Backing up OpenLDAP data ..."
    backup_database "${FILE_PREFIX}${OPENLDAP_CTL_BACKUP_DATA_FILE_SUFFIX}" 1
fi

if [ -n "${CLEAN_DAYS}" ]; then
    container log info "Cleaning up backups${CLEAN_PREFIX:+ starting with ${CLEAN_PREFIX}} older than ${CLEAN_DAYS} day(s) ..."
    clean "${CLEAN_DAYS}" "${CLEAN_PREFIX}"
fi
