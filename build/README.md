# CI/CD tool

Built with [osixia/dagger-build](https://github.com/osixia/dagger-build).

## Get Help
```
go run main.go --help
```

## Run Tests
```
go mod vendor
go run main.go job test ../Dockerfile
```

Limit a local run with `--arches amd64`.

After the shared version check, `tests/` runs integration scenarios on the image built for
the engine platform:

- default bootstrap;
- alternative schemas (`nis`, `virtualMail`);
- all features with 3 replicated nodes: all compatible modules and schemas, TLS required,
  readonly users, monitor, ppolicy, unique, refint, memberof and multi-provider replication;
- `openldap-ctl` backup and restore;
- upgrade: pre-upgrade backup on a simulated N-1 image, restore on N, downgrade refused;
- upgrade from previous published releases (`previousReleases` in `tests/upgrade.go`).

Scenarios are written in Go and run their LDAP checks with the scripts in
`tests/testdata/`. Replication nodes run with root capabilities in derived test containers,
to get their hostname (see `tests/testdata/node.sh`).
