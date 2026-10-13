# Changelog

Project-specific changes only. See upstream [OpenLDAP release notes](https://www.openldap.org/software/release/changes_lts.html).

## 2.6.15-beta

### Changed

- Add the Postfix Book schema in both OpenLDAP `.schema` and `cn=config` `.ldif` formats.
- Improve debug logging for file moves and schema conversion, including a check that the generated LDIF file exists.
- Update the base image to `osixia/baseimage:alpine-2.0.0-rc3`.
- Allow authenticated users to read `entry`, `objectClass`, `ou`, `o` and `dc` in addition to `cn`, `sn`, `givenName`, `mail` and `uid` in the default database ACL (new bootstraps only).
- Fail bootstrap with an explicit error when `OPENLDAP_BOOTSTRAP_SUFFIX` does not start with `dc=`, instead of creating a root entry with an empty `dc`.
- Fail bootstrap with an explicit error when `OPENLDAP_BOOTSTRAP_REPLICATION_HOSTS` contains more than 9 hosts.
- Check that backup files exist and are valid gzip archives before `openldap-ctl restore` stops OpenLDAP or deletes existing data.
- Print usage when `openldap-ctl schema2ldif` is called without arguments.

### Fixed

- Fix downgrade detection at the `minor` and `patch` migration levels.
- Fix `openldap-ctl password hash` for passwords containing spaces or shell special characters.
- Fix the default database ACL so authenticated users can actually read the allowed attributes.
- Remove bootstrap temporary files, including on failure.
- Strip leading and trailing empty lines from every bootstrap LDIF fragment, including the first file of each prefix group.

### Documentation

- Document the Postfix Book schema and its incompatibility with the `virtualMail` schema.
- Improve the schema conversion example for local files and non-root execution.


## 2.6.15-alpha

### Changed

- Upgrade OpenLDAP from 2.6.10 to 2.6.15.
- Upgrade base image to `osixia/baseimage:alpine-2.0.0-rc2`.
- Update `osixia/container-baseimage/build` and Go dependencies.
- Update GitHub Actions dependencies and use Go 1.26.1 in CI.

### Fixed

- Adjust SBOM attestation generation to use Cosign's `spdxjson` predicate type.

### Documentation

- Expand custom bootstrap LDIF documentation with volume mounts, template usage, and examples for adding, replacing, or removing built-in fragments.
- Clarify LDIF prefix grouping rules, including filenames without numeric prefixes and prefixes with leading zeros.
- Clarify that the cron service is disabled by default and document scheduled backups using a separate container.


## 2.6.10-alpha

⚠️ Breaking change: this version is a complete rewrite with a new base image and is not backward compatible. Please refer to the [README](README.md) for the new usage.

### Changed
  - Upgrade OpenLDAP version to 2.6.15
  - Upgrade base image to osixia/baseimage:alpine-2.0.0-rc2
  - Use GitHub action for CI/CD and osixia/container-baseimage/build tool

For previous releases, see the GitHub [releases page](https://github.com/osixia/container-openldap/releases).
