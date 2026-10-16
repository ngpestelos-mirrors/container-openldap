package tests

import (
	"context"
	"crypto/rand"
	_ "embed"
	"fmt"
	"log"
	"slices"
	"strings"

	"dagger.io/dagger"
)

const (
	suffix   = "dc=example,dc=org"
	password = "Test-passw0rd-2026"

	confDir   = "/etc/openldap/slapd.d"
	dataDir   = "/var/lib/openldap/openldap-data"
	backupDir = "/var/lib/openldap/openldap-backups"
	certsDir  = "/container/services/openldap/assets/certs"

	ldapUser = "ldap"
)

// env is a set of environment variables, applied in a stable order to keep Dagger caching
// deterministic.
type env map[string]string

func withEnv(c *dagger.Container, e env) *dagger.Container {
	keys := make([]string, 0, len(e))
	for k := range e {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	for _, k := range keys {
		c = c.WithEnvVariable(k, e[k])
	}

	return c
}

// merge returns a new env with the values of all given envs, later ones win.
func merge(envs ...env) env {
	r := env{}
	for _, e := range envs {
		for k, v := range e {
			r[k] = v
		}
	}
	return r
}

// volumes holds the OpenLDAP config, data and backup directories, shared between containers
// like Docker or Kubernetes volumes.
type volumes struct {
	conf   *dagger.Directory
	data   *dagger.Directory
	backup *dagger.Directory
}

func emptyVolumes(client *dagger.Client) volumes {
	return volumes{conf: client.Directory(), data: client.Directory(), backup: client.Directory()}
}

func withVolumes(c *dagger.Container, v volumes) *dagger.Container {
	opts := dagger.ContainerWithDirectoryOpts{Owner: ldapUser}
	return c.WithUser("0").
		WithDirectory(confDir, v.conf, opts).
		WithDirectory(dataDir, v.data, opts).
		WithDirectory(backupDir, v.backup, opts).
		WithUser(ldapUser)
}

func readVolumes(c *dagger.Container) volumes {
	return volumes{conf: c.Directory(confDir), data: c.Directory(dataDir), backup: c.Directory(backupDir)}
}

// result is the outcome of a command that is allowed to fail.
type result struct {
	container *dagger.Container
	code      int
	output    string
}

// run executes args in c, without failing on a non-zero exit code. A random value busts the
// Dagger cache so the command really runs on each test run.
func run(ctx context.Context, c *dagger.Container, args ...string) (*result, error) {

	c = c.WithEnvVariable("OPENLDAP_TEST_RUN", rand.Text()).
		WithExec(args, dagger.ContainerWithExecOpts{Expect: dagger.ReturnTypeAny})

	code, err := c.ExitCode(ctx)
	if err != nil {
		return nil, fmt.Errorf("%v: %w", args, err)
	}
	stdout, err := c.Stdout(ctx)
	if err != nil {
		return nil, fmt.Errorf("%v stdout: %w", args, err)
	}
	stderr, err := c.Stderr(ctx)
	if err != nil {
		return nil, fmt.Errorf("%v stderr: %w", args, err)
	}

	return &result{container: c, code: code, output: stdout + stderr}, nil
}

// expect runs args in c and checks its exit status and that its output contains every
// wanted string.
func expect(ctx context.Context, c *dagger.Container, success bool, want []string, args ...string) (*result, error) {

	r, err := run(ctx, c, args...)
	if err != nil {
		return nil, err
	}

	if success && r.code != 0 {
		return nil, fmt.Errorf("%v: exit code %d, expected success\n%s", args, r.code, r.output)
	}
	if !success && r.code == 0 {
		return nil, fmt.Errorf("%v: succeeded, expected failure\n%s", args, r.output)
	}

	for _, w := range want {
		if !strings.Contains(r.output, w) {
			return nil, fmt.Errorf("%v: output does not contain %q\n%s", args, w, r.output)
		}
	}

	return r, nil
}

// hashPassword hashes password with the image's openldap-ctl and its default {ARGON2} scheme.
func hashPassword(ctx context.Context, image *dagger.Container) (string, error) {
	return hashPasswordWith(ctx, image, "{ARGON2}", nil)
}

// hashPasswordWith hashes password with the image's openldap-ctl, e is applied to change
// the hash command.
func hashPasswordWith(ctx context.Context, image *dagger.Container, scheme string, e env) (string, error) {

	out, err := withEnv(image, e).WithExec([]string{"container", "run", "--", "openldap-ctl", "password", "hash", password}).Stdout(ctx)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	hash := strings.TrimSpace(out)
	if !strings.HasPrefix(hash, scheme) {
		return "", fmt.Errorf("unexpected password hash: %q", hash)
	}

	return hash, nil
}

// certificates generates a test CA and a server certificate valid for hosts.
func certificates(client *dagger.Client, hosts []string) *dagger.Directory {

	sans := make([]string, 0, len(hosts))
	for _, h := range hosts {
		sans = append(sans, "DNS:"+h)
	}

	// a random value busts the Dagger cache, so certificates never expire between runs
	work := client.Container().From("alpine:3.24").
		WithExec([]string{"apk", "add", "--no-cache", "openssl"}).
		WithEnvVariable("OPENLDAP_TEST_RUN", rand.Text()).
		WithWorkdir("/work").
		WithNewFile("/work/san.ext", "subjectAltName="+strings.Join(sans, ",")+"\n").
		WithExec([]string{"openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "2", "-subj", "/CN=osixia-openldap-test-ca", "-keyout", "ca.key", "-out", "ca.crt"}).
		WithExec([]string{"openssl", "req", "-newkey", "rsa:2048", "-nodes", "-subj", "/CN=" + hosts[0], "-keyout", "tls.key", "-out", "tls.csr"}).
		WithExec([]string{"openssl", "x509", "-req", "-days", "2", "-in", "tls.csr", "-CA", "ca.crt", "-CAkey", "ca.key", "-CAcreateserial", "-extfile", "san.ext", "-out", "tls.crt"})

	return client.Directory().
		WithFile("ca.crt", work.File("/work/ca.crt")).
		WithFile("tls.crt", work.File("/work/tls.crt")).
		WithFile("tls.key", work.File("/work/tls.key"))
}

// startNode starts the image as a service reachable at fqdn.
//
// fqdn must not contain dots: Dagger registers it under the session search domain, which
// musl only uses for names without dots.
//
// The node.sh fixture gives the container this hostname and makes it resolve locally, as
// Docker or Kubernetes do: the replication bootstrap looks for `hostname -f` in
// OPENLDAP_BOOTSTRAP_REPLICATION_HOSTS and slapd matches olcServerID URLs against its
// listeners. This needs root capabilities in this derived test container only, then
// the image runs as the ldap user.
func startNode(ctx context.Context, image *dagger.Container, fqdn string, e env, certs *dagger.Directory, v *volumes) (*dagger.Service, error) {

	c := image
	if v != nil {
		c = withVolumes(c, *v)
	}
	if certs != nil {
		c = c.WithUser("0").WithDirectory(certsDir, certs, dagger.ContainerWithDirectoryOpts{Owner: ldapUser})
	}

	service := withEnv(c, e).
		WithUser("0").
		WithNewFile("/tmp/openldap-test-node.sh", nodeScript, dagger.ContainerWithNewFileOpts{Permissions: 0755}).
		WithExposedPort(3890).
		WithExposedPort(6360).
		AsService(dagger.ContainerAsServiceOpts{
			Args:                     []string{"/tmp/openldap-test-node.sh", fqdn},
			InsecureRootCapabilities: true,
		}).
		WithHostname(fqdn)

	if _, err := service.Start(ctx); err != nil {
		return nil, fmt.Errorf("start %s: %w", fqdn, err)
	}

	return service, nil
}

func stopNodes(ctx context.Context, services []*dagger.Service) {
	for _, s := range services {
		_, _ = s.Stop(ctx, dagger.ServiceStopOpts{Kill: true})
	}
}

// check runs a testdata script in a client container built from the image, with the test
// CA trusted.
func check(ctx context.Context, image *dagger.Container, certs *dagger.Directory, e env, script string) error {

	c := image
	if certs != nil {
		c = c.WithMountedDirectory("/tmp/openldap-test-certs", certs).
			WithEnvVariable("LDAPTLS_CACERT", "/tmp/openldap-test-certs/ca.crt")
	}

	c = withEnv(c, merge(env{"TEST_PASSWORD": password, "TEST_SUFFIX": suffix}, e)).
		WithNewFile("/tmp/openldap-test-lib.sh", libScript).
		WithNewFile("/tmp/openldap-test.sh", script)

	r, err := expect(ctx, c, true, nil, "bash", "/tmp/openldap-test.sh")
	if err != nil {
		return err
	}

	for line := range strings.Lines(r.output) {
		if strings.HasPrefix(line, "ok: ") {
			log.Print(strings.TrimSpace(line))
		}
	}

	return nil
}

//go:embed testdata/node.sh
var nodeScript string

//go:embed testdata/lib.sh
var libScript string
