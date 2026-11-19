package provider

import "sync"

// accessMu serializes API calls that make Forgejo recalculate repository
// access rows (collaborator, team and team membership changes).
//
// Forgejo rebuilds the access table of a repository by deleting and
// re-inserting its rows. When Terraform runs several such changes in parallel,
// overlapping rebuilds can fail with a unique constraint violation
// ("UQE_access_s"). Holding this lock around the mutating call avoids that
// within a single provider process.
var accessMu sync.Mutex
