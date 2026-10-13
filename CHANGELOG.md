# Changelog

Project-specific changes only. See upstream [OpenLDAP release notes](https://www.openldap.org/software/release/changes_lts.html).

## 2.6.15-beta

### Changed

- Add the Postfix Book schema in both OpenLDAP `.schema` and `cn=config` `.ldif` formats.
- Improve debug logging for file moves and schema conversion, including a check that the generated LDIF file exists.
- Update the base image to `osixia/baseimage:alpine-2.0.0-rc3`.

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
