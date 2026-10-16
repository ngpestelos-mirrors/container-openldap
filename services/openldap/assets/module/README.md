# OpenLDAP Modules

This directory contains optional modules that can be loaded during bootstrap with `OPENLDAP_BOOTSTRAP_MODULES`.

## Available modules

All OpenLDAP backends and overlays from the Alpine packages are in `/usr/lib/openldap`:

`accesslog.so`, `argon2.so`, `auditlog.so`, `autoca.so`, `back_asyncmeta.so`, `back_dnssrv.so`, `back_ldap.so`, `back_mdb.so`, `back_meta.so`, `back_null.so`, `back_passwd.so`, `back_relay.so`, `back_sock.so`, `back_sql.so`, `collect.so`, `constraint.so`, `dds.so`, `deref.so`, `dyngroup.so`, `dynlist.so`, `homedir.so`, `lastbind.so`, `lloadd.so`, `memberof.so`, `mqtt.so`, `nestgroup.so`, `otp.so`, `pcache.so`, `ppolicy.so`, `pw-pbkdf2.so`, `pw-sha2.so`, `refint.so`, `remoteauth.so`, `retcode.so`, `rwm.so`, `seqmod.so`, `sssvlv.so`, `syncprov.so`, `translucent.so`, `unique.so`, `valsort.so`

Incompatibilities, all other modules can be loaded together:

- `dyngroup.so` and `dynlist.so`;
- `lastbind.so` and `mqtt.so`.
