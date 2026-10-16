package tests

import (
	"context"
	"fmt"
	"log"
	"time"

	"dagger.io/dagger"

	"github.com/osixia/dagger-build/job"
)

// scenario is an integration test run against a built image.
type scenario struct {
	name string
	run  func(ctx context.Context, client *dagger.Client, image *dagger.Container) error
}

var scenarios = []scenario{
	{"default bootstrap", Default},
	{"alternative schemas", AlternativeSchemas},
	{"all features with 3 replicated nodes", AllFeatures},
	{"backup and restore", BackupRestore},
	{"upgrade", Upgrade},
	{"upgrade from previous releases", UpgradeFromPreviousReleases},
}

// Run checks OpenLDAP behavior after the reusable job's version check, only on the engine
// platform: emulated images are slow and fail as Dagger services.
func Run(ctx context.Context, client *dagger.Client, builds []*job.BuildResult) error {

	enginePlatform, err := client.DefaultPlatform(ctx)
	if err != nil {
		return fmt.Errorf("engine platform: %w", err)
	}

	tested := 0
	for _, build := range builds {
		for _, image := range build.Containers {

			platform, err := image.Platform(ctx)
			if err != nil {
				return fmt.Errorf("image platform: %w", err)
			}
			if platform != enginePlatform {
				continue
			}
			tested++

			for _, s := range scenarios {
				log.Printf("scenario: %s ...", s.name)
				sctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
				err := s.run(sctx, client, image)
				cancel()
				if err != nil {
					return fmt.Errorf("%s: %w", s.name, err)
				}
				log.Printf("scenario: %s passed", s.name)
			}
		}
	}

	if tested == 0 && len(builds) > 0 {
		return fmt.Errorf("no image built for the engine platform %v", enginePlatform)
	}

	return nil
}
