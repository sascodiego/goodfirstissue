package store

import "testing"

func TestRegistryStartsEmpty(t *testing.T) {
	s := newStore(t)
	repos, err := s.Repos()
	if err != nil {
		t.Fatalf("Repos() error = %v", err)
	}
	if len(repos) != 0 {
		t.Fatalf("Repos() = %v, want none", repos)
	}
}

// The list of repositories is the requirement: it must survive a restart.
func TestRepoListSurvivesReopeningTheStore(t *testing.T) {
	s := newStore(t)
	if err := s.SaveRepo(RepoEntry{
		Name: "spf13/cobra", Key: "spf13__cobra", CommitSHA: "abc123", TotalOpen: 262, Issues: 150,
	}); err != nil {
		t.Fatalf("SaveRepo() error = %v", err)
	}

	reopened, err := New(s.Dir())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	repos, err := reopened.Repos()
	if err != nil {
		t.Fatalf("Repos() error = %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("Repos() = %d entries, want 1", len(repos))
	}
	got := repos[0]
	if got.Name != "spf13/cobra" || got.CommitSHA != "abc123" || got.Issues != 150 {
		t.Errorf("entry = %+v", got)
	}
	if got.FetchedAt.IsZero() {
		t.Error("SaveRepo did not stamp the fetch time")
	}
}

func TestSaveRepoUpdatesInsteadOfDuplicating(t *testing.T) {
	s := newStore(t)
	if err := s.SaveRepo(RepoEntry{Name: "a/b", Key: "a__b", CommitSHA: "old", Issues: 10}); err != nil {
		t.Fatalf("SaveRepo() error = %v", err)
	}
	if err := s.SaveRepo(RepoEntry{Name: "a/b", Key: "a__b", CommitSHA: "new", Issues: 20}); err != nil {
		t.Fatalf("SaveRepo() error = %v", err)
	}

	repos, err := s.Repos()
	if err != nil {
		t.Fatalf("Repos() error = %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("Repos() = %d entries, want 1", len(repos))
	}
	if repos[0].CommitSHA != "new" || repos[0].Issues != 20 {
		t.Errorf("entry = %+v, want the newer fetch", repos[0])
	}
}

func TestReposAreOrderedByName(t *testing.T) {
	s := newStore(t)
	for _, e := range []RepoEntry{
		{Name: "zeta/z", Key: "zeta__z"},
		{Name: "alpha/a", Key: "alpha__a"},
		{Name: "mid/m", Key: "mid__m"},
	} {
		if err := s.SaveRepo(e); err != nil {
			t.Fatalf("SaveRepo(%s) error = %v", e.Key, err)
		}
	}
	repos, err := s.Repos()
	if err != nil {
		t.Fatalf("Repos() error = %v", err)
	}
	want := []string{"alpha/a", "mid/m", "zeta/z"}
	for i, name := range want {
		if repos[i].Name != name {
			t.Errorf("repos[%d] = %s, want %s", i, repos[i].Name, name)
		}
	}
}

func TestRepoEntryNeedsAKey(t *testing.T) {
	s := newStore(t)
	if err := s.SaveRepo(RepoEntry{Name: "a/b"}); err == nil {
		t.Fatal("an entry without a key was accepted")
	}
}

func TestRepoFindsByKey(t *testing.T) {
	s := newStore(t)
	if err := s.SaveRepo(RepoEntry{Name: "spf13/cobra", Key: "spf13__cobra"}); err != nil {
		t.Fatalf("SaveRepo() error = %v", err)
	}
	got, ok, err := s.Repo("spf13__cobra")
	if err != nil || !ok {
		t.Fatalf("Repo() = %+v,%v,%v", got, ok, err)
	}
	if got.Name != "spf13/cobra" {
		t.Errorf("name = %s", got.Name)
	}
	if _, ok, err := s.Repo("nope__nope"); err != nil || ok {
		t.Errorf("Repo(unknown) = %v,%v, want false and no error", ok, err)
	}
}

// A renamed repository must not appear twice: the old key has to be dropped, and asking to
// drop a key that is not there must not be an error. The second half matters because the
// caller does it on every fetch that GitHub answers with a different name.
func TestRemoveRepoDropsOnlyTheGivenKey(t *testing.T) {
	s := newStore(t)
	for _, e := range []RepoEntry{
		{Name: "httpie/httpie", Key: "httpie__httpie"},
		{Name: "spf13/cobra", Key: "spf13__cobra"},
	} {
		if err := s.SaveRepo(e); err != nil {
			t.Fatalf("SaveRepo(%s) error = %v", e.Key, err)
		}
	}

	if err := s.RemoveRepo("httpie__httpie"); err != nil {
		t.Fatalf("RemoveRepo() error = %v", err)
	}
	repos, err := s.Repos()
	if err != nil {
		t.Fatalf("Repos() error = %v", err)
	}
	if len(repos) != 1 || repos[0].Key != "spf13__cobra" {
		t.Fatalf("Repos() = %+v, want only spf13__cobra", repos)
	}

	if err := s.RemoveRepo("nunca__existio"); err != nil {
		t.Errorf("removing an absent key = %v, want no error", err)
	}
	if err := s.RemoveRepo(""); err == nil {
		t.Error("removing the empty key = nil, want an error")
	}
}
