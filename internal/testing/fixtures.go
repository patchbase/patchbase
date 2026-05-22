package testing

import (
	"context"
	sqldb "database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-testfixtures/testfixtures/v3"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func LoadYAMLFixtures(paths ...string) Fixture {
	return func(_ context.Context, backend *Backend) error {
		if len(paths) == 0 {
			return nil
		}

		resolved := make([]string, 0, len(paths))
		for _, path := range paths {
			filePath, err := resolveFixtureFile(path)
			if err != nil {
				return err
			}
			resolved = append(resolved, filePath)
		}

		db, err := sqldb.Open("pgx", backend.Config().Database.URL)
		if err != nil {
			return fmt.Errorf("open fixture database connection: %w", err)
		}
		defer db.Close() // nolint:errcheck

		loader, err := testfixtures.New(
			testfixtures.Database(db),
			testfixtures.Dialect("postgres"),
			testfixtures.Files(resolved...),
			testfixtures.UseAlterConstraint(),
		)
		if err != nil {
			return fmt.Errorf("create fixture loader: %w", err)
		}

		if err := loader.Load(); err != nil {
			return fmt.Errorf("load fixtures: %w", err)
		}

		return nil
	}
}

func resolveFixtureFile(path string) (string, error) {
	for _, candidate := range fixtureFileCandidates(path) {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("fixture file not found: %s", path)
}

func fixtureFileCandidates(path string) []string {
	candidates := []string{
		path,
		filepath.Join("db", "fixtures", path),
	}

	if testSrcDir := os.Getenv("TEST_SRCDIR"); testSrcDir != "" {
		workspace := os.Getenv("TEST_WORKSPACE")
		if workspace != "" {
			candidates = append(candidates, filepath.Join(testSrcDir, workspace, "db", "fixtures", path))
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return candidates
	}

	dir := cwd
	for {
		candidates = append(candidates, filepath.Join(dir, "db", "fixtures", path))
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return candidates
}
