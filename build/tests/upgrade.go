package tests

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"dagger.io/dagger"
)

// olderVersion is lower than the image OpenLDAP version at the patch level and equal at the
// minor level, to simulate the N-1 image with the N image.
const olderVersion = "2.6.0-r0"

// previousRelease is a published image to upgrade from.
type previousRelease struct {
	image string

	// manualBackup takes the pre-upgrade backup with openldap-ctl: the openldap-pre-upgrade
	// service of 2.6.10-alpha is not executable.
	manualBackup bool
}

var previousReleases = []previousRelease{
	{image: "osixia/openldap:2.6.10-alpha", manualBackup: true},
	{image: "osixia/openldap:2.6.15-alpha"},
}

const peopleLDIF = `dn: ou=people,` + suffix + `
objectClass: organizationalUnit
ou: people

dn: uid=alice,ou=people,` + suffix + `
objectClass: inetOrgPerson
uid: alice
cn: Alice
sn: Liddell
`

// bootstrap runs the startup scripts of image on v, with the test root passwords, then adds
// peopleLDIF offline.
func bootstrap(ctx context.Context, image *dagger.Container, v volumes, passwords env) (volumes, error) {

	r, err := expect(ctx, withEnv(withVolumes(image, v), passwords), true, []string{"Writing OpenLDAP version"}, "container", "--skip-process")
	if err != nil {
		return v, fmt.Errorf("bootstrap: %w", err)
	}

	r, err = expect(ctx, r.container.WithNewFile("/tmp/openldap-test-people.ldif", peopleLDIF), true, nil,
		"container", "run", "--", "slapadd", "-n", "1", "-l", "/tmp/openldap-test-people.ldif")
	if err != nil {
		return v, fmt.Errorf("add test entries: %w", err)
	}

	return readVolumes(r.container), nil
}

// expectPeople checks peopleLDIF entries are in the database of v, offline.
func expectPeople(ctx context.Context, image *dagger.Container, v volumes) error {
	_, err := expect(ctx, withVolumes(image, v), true, []string{"dn: uid=alice,ou=people," + suffix},
		"container", "run", "--", "slapcat", "-n", "1")
	return err
}

// expectVersion checks the version file of v.
func expectVersion(ctx context.Context, image *dagger.Container, v volumes, version string) error {
	_, err := expect(ctx, withVolumes(image, v), true, []string{version}, "cat", confDir+"/.version")
	return err
}

// expectServed starts image on v and checks peopleLDIF entries are served.
func expectServed(ctx context.Context, image *dagger.Container, v volumes) error {

	const node = "ldap-upgraded"

	service, err := startNode(ctx, image, node, nil, nil, &v)
	if err != nil {
		return err
	}
	defer stopNodes(ctx, []*dagger.Service{service})

	return check(ctx, image, nil, env{"TEST_NODES": node}, servedCheck)
}

// BackupRestore checks openldap-ctl backup and restore, offline.
func BackupRestore(ctx context.Context, client *dagger.Client, image *dagger.Container) error {

	passwords, err := rootPasswords(ctx, image)
	if err != nil {
		return err
	}

	v, err := bootstrap(ctx, image, emptyVolumes(client), passwords)
	if err != nil {
		return err
	}

	r, err := expect(ctx, withVolumes(image, v), true, nil, "container", "run", "--", "openldap-ctl", "backup", "test-backup")
	if err != nil {
		return err
	}
	v = readVolumes(r.container)

	if _, err := expect(ctx, withVolumes(image, v), true, []string{"test-backup-config.gz", "test-backup-data.gz"}, "ls", backupDir); err != nil {
		return err
	}

	if err := clean(ctx, image, v); err != nil {
		return err
	}

	// restore into empty config and data directories
	restored := emptyVolumes(client)
	restored.backup = v.backup

	r, err = expect(ctx, withVolumes(image, restored), true, nil, "container", "run", "--", "openldap-ctl", "restore", "test-backup")
	if err != nil {
		return err
	}
	if err := expectPeople(ctx, image, readVolumes(r.container)); err != nil {
		return fmt.Errorf("restore into empty directories: %w", err)
	}

	// restore over existing data requires --force
	if _, err := expect(ctx, withVolumes(image, v), false, nil, "container", "run", "--", "openldap-ctl", "restore", "test-backup"); err != nil {
		return fmt.Errorf("restore over existing data without --force: %w", err)
	}

	r, err = expect(ctx, withVolumes(image, v), true, nil, "container", "run", "--", "openldap-ctl", "restore", "test-backup", "--force")
	if err != nil {
		return err
	}
	if err := expectPeople(ctx, image, readVolumes(r.container)); err != nil {
		return fmt.Errorf("forced restore: %w", err)
	}

	// an invalid backup is rejected before existing data is deleted
	corrupted := v
	corrupted.backup = v.backup.
		WithNewFile("corrupted-config.gz", "not a gzip file").
		WithNewFile("corrupted-data.gz", "not a gzip file")

	r, err = expect(ctx, withVolumes(image, corrupted), false, nil, "container", "run", "--", "openldap-ctl", "restore", "corrupted", "--force")
	if err != nil {
		return fmt.Errorf("restore of a corrupted backup: %w", err)
	}
	if err := expectPeople(ctx, image, readVolumes(r.container)); err != nil {
		return fmt.Errorf("data after the restore of a corrupted backup: %w", err)
	}

	// a backup that fails to load is rolled back to the previous data
	invalid := withVolumes(image, v).
		WithExec([]string{"cp", backupDir + "/test-backup-config.gz", backupDir + "/invalid-config.gz"}).
		WithExec([]string{"sh", "-c", "printf 'dn: not a valid entry\\nfoo\\n' | gzip > " + backupDir + "/invalid-data.gz"})

	r, err = expect(ctx, invalid, false, []string{"Restore failed, rolling back to the previous data"}, "container", "run", "--", "openldap-ctl", "restore", "invalid", "--force")
	if err != nil {
		return fmt.Errorf("restore of an invalid backup: %w", err)
	}
	rolledBack := readVolumes(r.container)
	if err := expectPeople(ctx, image, rolledBack); err != nil {
		return fmt.Errorf("data after the rollback: %w", err)
	}
	if _, err := expect(ctx, withVolumes(image, rolledBack), false, nil, "ls", "-d", confDir+"/.restore-previous", dataDir+"/.restore-previous"); err != nil {
		return fmt.Errorf("previous data directories after the rollback: %w", err)
	}

	// an interrupted restore is detected
	interrupted := withVolumes(image, v).WithExec([]string{"mkdir", dataDir + "/.restore-previous"})
	if _, err := expect(ctx, interrupted, false, []string{"a previous restore was interrupted"}, "container", "run", "--", "openldap-ctl", "restore", "test-backup", "--force"); err != nil {
		return fmt.Errorf("restore after an interrupted restore: %w", err)
	}

	return nil
}

// Upgrade simulates the N-1 image with olderVersion and checks the pre-upgrade backup on
// N-1 and the restore-based migration on N.
func Upgrade(ctx context.Context, client *dagger.Client, image *dagger.Container) error {

	version, err := image.EnvVariable(ctx, "OPENLDAP_VERSION")
	if err != nil {
		return err
	}

	passwords, err := rootPasswords(ctx, image)
	if err != nil {
		return err
	}

	patch := env{"OPENLDAP_UPGRADE_MIGRATION_LEVEL": "patch"}
	newer := withEnv(image, patch)
	older := withEnv(image, merge(patch, env{"OPENLDAP_VERSION": olderVersion}))

	v, err := bootstrap(ctx, older, emptyVolumes(client), passwords)
	if err != nil {
		return err
	}

	// N without pre-upgrade backup
	if _, err := expect(ctx, withVolumes(newer, v), false, []string{"Missing"}, "container", "--skip-process"); err != nil {
		return fmt.Errorf("upgrade without backup: %w", err)
	}

	// the default minor migration level ignores patch versions
	if _, err := expect(ctx, withVolumes(image, v), true, []string{"No migration needed"}, "container", "--skip-process"); err != nil {
		return fmt.Errorf("upgrade at the minor level: %w", err)
	}

	// N-1 pre-upgrade, as an init container
	backupPrefix := "openldap-upgrade-" + strings.TrimSuffix(olderVersion, "-r0")
	r, err := expect(ctx, withVolumes(older, v), true, nil, "container", "-x", "openldap-pre-upgrade", "--skip-process")
	if err != nil {
		return fmt.Errorf("pre-upgrade: %w", err)
	}
	v = readVolumes(r.container)

	if _, err := expect(ctx, withVolumes(image, v), true, []string{backupPrefix + "-config.gz", backupPrefix + "-data.gz"}, "ls", backupDir); err != nil {
		return fmt.Errorf("pre-upgrade backup: %w", err)
	}

	// N
	r, err = expect(ctx, withVolumes(newer, v), true, []string{"Migration required"}, "container", "--skip-process")
	if err != nil {
		return fmt.Errorf("upgrade: %w", err)
	}
	v = readVolumes(r.container)

	if err := expectVersion(ctx, image, v, version); err != nil {
		return err
	}
	if err := expectPeople(ctx, image, v); err != nil {
		return err
	}

	// N-1 again
	if _, err := expect(ctx, withVolumes(older, v), false, []string{"Downgrade forbidden"}, "container", "--skip-process"); err != nil {
		return fmt.Errorf("downgrade: %w", err)
	}

	return expectServed(ctx, image, v)
}

// UpgradeFromPreviousReleases upgrades data created by published images.
func UpgradeFromPreviousReleases(ctx context.Context, client *dagger.Client, image *dagger.Container) error {

	version, err := image.EnvVariable(ctx, "OPENLDAP_VERSION")
	if err != nil {
		return err
	}

	patch := env{"OPENLDAP_UPGRADE_MIGRATION_LEVEL": "patch"}

	// 2.6.10-alpha expands "$" in hashed root passwords: use a {SSHA} hash, without "$"
	hash, err := hashPasswordWith(ctx, image, "{SSHA}", env{"OPENLDAP_CTL_PASSWORD_HASH_CMD": "slappasswd -h {SSHA}"})
	if err != nil {
		return err
	}
	passwords := env{
		"OPENLDAP_BOOTSTRAP_CONFIG_ROOT_PASSWORD_HASHED": hash,
		"OPENLDAP_BOOTSTRAP_DATA_ROOT_PASSWORD_HASHED":   hash,
	}

	for _, p := range previousReleases {

		previous := withEnv(client.Container().From(p.image), patch)

		previousVersion, err := previous.EnvVariable(ctx, "OPENLDAP_VERSION")
		if err != nil {
			return fmt.Errorf("%s: %w", p.image, err)
		}

		v, err := bootstrap(ctx, previous, emptyVolumes(client), passwords)
		if err != nil {
			return fmt.Errorf("%s: %w", p.image, err)
		}

		preUpgrade := []string{"container", "-x", "openldap-pre-upgrade", "--skip-process"}
		if p.manualBackup {
			preUpgrade = []string{"container", "run", "--", "openldap-ctl", "backup", "openldap-upgrade-" + strings.Split(previousVersion, "-")[0]}
		}

		r, err := expect(ctx, withVolumes(previous, v), true, nil, preUpgrade...)
		if err != nil {
			return fmt.Errorf("%s pre-upgrade: %w", p.image, err)
		}
		v = readVolumes(r.container)

		r, err = expect(ctx, withVolumes(withEnv(image, patch), v), true, nil, "container", "--skip-process")
		if err != nil {
			return fmt.Errorf("%s upgrade: %w", p.image, err)
		}
		v = readVolumes(r.container)

		if err := expectVersion(ctx, image, v, version); err != nil {
			return fmt.Errorf("%s: %w", p.image, err)
		}
		if err := expectPeople(ctx, image, v); err != nil {
			return fmt.Errorf("%s: %w", p.image, err)
		}
		if err := expectServed(ctx, image, v); err != nil {
			return fmt.Errorf("%s: %w", p.image, err)
		}
	}

	return nil
}

//go:embed testdata/served.sh
var servedCheck string

// clean checks openldap-ctl backup --clean only removes old backup files, starting with
// --clean-prefix if set.
func clean(ctx context.Context, image *dagger.Container, v volumes) error {

	old := []string{
		"openldap-cron-old-config.gz", "openldap-cron-old-data.gz",
		"manual-old-config.gz", "manual-old-data.gz",
		"openldap-upgrade-2.6-config.gz", "other-old-file.txt",
	}

	aged := withVolumes(image, v)
	for _, f := range old {
		aged = aged.WithExec([]string{"touch", "-d", "2000-01-01", backupDir + "/" + f})
	}

	ls := func(c *dagger.Container) (string, error) {
		r, err := expect(ctx, c, true, nil, "ls", backupDir)
		if err != nil {
			return "", err
		}
		return r.output, nil
	}

	r, err := expect(ctx, aged, true, nil, "container", "run", "--", "openldap-ctl", "backup", "openldap-cron-new", "--clean", "1", "--clean-prefix", "openldap-cron-")
	if err != nil {
		return err
	}
	files, err := ls(r.container)
	if err != nil {
		return err
	}
	for _, f := range []string{"openldap-cron-new-config.gz", "manual-old-config.gz", "openldap-upgrade-2.6-config.gz", "other-old-file.txt", "test-backup-config.gz"} {
		if !strings.Contains(files, f) {
			return fmt.Errorf("--clean-prefix removed %s:\n%s", f, files)
		}
	}
	if strings.Contains(files, "openldap-cron-old-") {
		return fmt.Errorf("--clean-prefix did not remove old cron backups:\n%s", files)
	}

	r, err = expect(ctx, r.container, true, nil, "container", "run", "--", "openldap-ctl", "backup", "manual-new", "--clean", "1")
	if err != nil {
		return err
	}
	files, err = ls(r.container)
	if err != nil {
		return err
	}
	if strings.Contains(files, "manual-old-") || strings.Contains(files, "openldap-upgrade-2.6-") {
		return fmt.Errorf("--clean did not remove old backups:\n%s", files)
	}
	if !strings.Contains(files, "other-old-file.txt") || !strings.Contains(files, "test-backup-config.gz") {
		return fmt.Errorf("--clean removed a file that is not an old backup:\n%s", files)
	}

	return nil
}
