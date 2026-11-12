package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v4"
)

// Forgejo stores allowlist entries (users and teams) of tag protections by
// ID, but its API only reports entries that currently have read access to the
// repository (see services/convert.ToTagProtection). An allowlisted team that
// has not been granted access to the repository, or a user who is not a
// reader, is therefore silently omitted from API responses even though it is
// stored and enforced.
//
// The helpers in this file keep such "unobservable" entries in state so that
// Terraform does not report an inconsistent result after apply or a
// permanent diff on refresh.

// allowlistReaderLookup resolves which teams and users have read access to a
// repository, lazily and at most once per lookup.
type allowlistReaderLookup struct {
	client *forgejo.Client
	owner  string
	repo   string

	teams      map[string]*forgejo.Team
	teamsErr   error
	teamsDone  bool
	users      map[string]bool
	personal   bool
	usersErr   error
	usersDone  bool
	readAccess map[forgejo.AccessMode]bool
}

func newAllowlistReaderLookup(client *forgejo.Client, owner, repo string) *allowlistReaderLookup {
	return &allowlistReaderLookup{
		client: client,
		owner:  owner,
		repo:   repo,
		readAccess: map[forgejo.AccessMode]bool{
			forgejo.AccessModeRead:  true,
			forgejo.AccessModeWrite: true,
			forgejo.AccessModeAdmin: true,
			forgejo.AccessModeOwner: true,
		},
	}
}

// readerTeams returns the teams with read access to the repository, keyed by name.
func (l *allowlistReaderLookup) readerTeams() (map[string]*forgejo.Team, error) {
	if l.teamsDone {
		return l.teams, l.teamsErr
	}
	l.teamsDone = true

	teams, res, err := l.client.GetRepoTeams(l.owner, l.repo)
	if err != nil && res != nil && res.StatusCode == 405 {
		// Personal repository: Forgejo has no teams to report.
		l.personal = true
		l.teams = map[string]*forgejo.Team{}
		return l.teams, nil
	}
	if err != nil {
		l.teamsErr = fmt.Errorf("listing teams of %s/%s: %w", l.owner, l.repo, err)
		return nil, l.teamsErr
	}

	l.teams = make(map[string]*forgejo.Team, len(teams))
	for _, t := range teams {
		if l.readAccess[t.Permission] {
			l.teams[t.Name] = t
		}
	}

	return l.teams, nil
}

// readerUsers returns the usernames with explicit read access to the
// repository: collaborators, members of teams with access, and the owner
// of a personal repository.
func (l *allowlistReaderLookup) readerUsers() (map[string]bool, error) {
	if l.usersDone {
		return l.users, l.usersErr
	}
	l.usersDone = true

	users := map[string]bool{}

	const pageSize = 50
	for page := 1; ; page++ {
		collaborators, _, err := l.client.ListCollaborators(l.owner, l.repo, forgejo.ListCollaboratorsOptions{
			ListOptions: forgejo.ListOptions{Page: page, PageSize: pageSize},
		})
		if err != nil {
			l.usersErr = fmt.Errorf("listing collaborators of %s/%s: %w", l.owner, l.repo, err)
			return nil, l.usersErr
		}
		for _, u := range collaborators {
			users[u.UserName] = true
		}
		if len(collaborators) < pageSize {
			break
		}
	}

	teams, err := l.readerTeams()
	if err != nil {
		l.usersErr = err
		return nil, l.usersErr
	}
	for _, t := range teams {
		for page := 1; ; page++ {
			members, _, err := l.client.ListTeamMembers(t.ID, forgejo.ListTeamMembersOptions{
				ListOptions: forgejo.ListOptions{Page: page, PageSize: pageSize},
			})
			if err != nil {
				l.usersErr = fmt.Errorf("listing members of team %q: %w", t.Name, err)
				return nil, l.usersErr
			}
			for _, u := range members {
				users[u.UserName] = true
			}
			if len(members) < pageSize {
				break
			}
		}
	}

	// The owner of a personal repository always counts as a reader.
	if l.personal {
		users[l.owner] = true
	}

	l.users = users

	return l.users, nil
}

// mergeAllowlist combines the entries reported by the API with entries from
// prior (plan or state) that the API cannot report. An entry in prior that
// is missing from reported is kept only if isReader reports that it lacks
// read access to the repository, i.e. Forgejo would hide it even if it is
// still on the allowlist. If isReader fails, the prior entry is kept, as
// it cannot be told apart from a hidden one.
func mergeAllowlist(ctx context.Context, attr string, prior types.Set, reported []string, isReader func(string) (bool, error)) types.Set {
	priorVals, err := setToStringSlice(ctx, prior)
	if err != nil || len(priorVals) == 0 {
		return stringSliceToSet(reported)
	}

	seen := make(map[string]bool, len(reported))
	for _, v := range reported {
		seen[v] = true
	}

	merged := append([]string{}, reported...)
	for _, v := range priorVals {
		if seen[v] {
			continue
		}
		if isReader != nil {
			reader, err := isReader(v)
			if err != nil {
				tflog.Warn(ctx, "Unable to determine whether allowlist entry is visible; keeping it in state", map[string]any{
					"attribute": attr,
					"entry":     v,
					"error":     err.Error(),
				})
			} else if reader {
				// Forgejo would have reported it, so it was really removed.
				continue
			}
		}
		tflog.Info(ctx, "Keeping allowlist entry that Forgejo does not report because it lacks repository read access", map[string]any{
			"attribute": attr,
			"entry":     v,
		})
		merged = append(merged, v)
		seen[v] = true
	}

	return stringSliceToSet(merged)
}

// isTeamReader reports whether the named team has read access to the repository.
func (l *allowlistReaderLookup) isTeamReader(name string) (bool, error) {
	teams, err := l.readerTeams()
	if err != nil {
		return false, err
	}
	_, ok := teams[name]
	return ok, nil
}

// isUserReader reports whether the named user has explicit read access to the repository.
func (l *allowlistReaderLookup) isUserReader(name string) (bool, error) {
	users, err := l.readerUsers()
	if err != nil {
		return false, err
	}
	return users[name], nil
}

// droppedAllowlistEntries returns the entries of planned that are missing
// from reported. Names are compared case-insensitively, as Forgejo does.
func droppedAllowlistEntries(ctx context.Context, planned types.Set, reported []string) []string {
	plannedVals, err := setToStringSlice(ctx, planned)
	if err != nil {
		return nil
	}

	seen := make(map[string]bool, len(reported))
	for _, v := range reported {
		seen[strings.ToLower(v)] = true
	}

	var dropped []string
	for _, v := range plannedVals {
		if !seen[strings.ToLower(v)] {
			dropped = append(dropped, v)
		}
	}

	return dropped
}
