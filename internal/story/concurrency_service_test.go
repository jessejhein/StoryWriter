package story

import (
	"context"
	"testing"
	"time"

	"storywork/internal/project"
)

// BDD trace:
//   - Requirement: Milestone 1 mutation rule, serialize structural mutations.
//   - Scenario: when two structural requests overlap, the second cannot load or
//     write canonical state until the first mutation has completed.
//   - Test purpose: verify the service lock prevents concurrent read-modify-write
//     cycles that could overwrite one another.
func TestStructuralMutationsAreSerialized(t *testing.T) {
	t.Parallel()

	firstLoadEntered := make(chan struct{})
	releaseFirstLoad := make(chan struct{})
	files := &fakeFileStore{exists: map[string]bool{}}
	files.loadHook = func(call int) {
		if call == 1 {
			close(firstLoadEntered)
			<-releaseFirstLoad
		}
	}
	service := NewService(
		&fakeSession{current: project.Project{Path: "/tmp/story"}, ok: true},
		files,
		&fakeGitStore{clean: true},
		&fakeIndexStore{},
		&fakeIDGenerator{ids: []string{"arc_00000000000000000001", "arc_00000000000000000002"}},
	)

	firstDone := make(chan error, 1)
	go func() {
		_, err := service.CreateArc(context.Background(), "One")
		firstDone <- err
	}()
	<-firstLoadEntered

	secondDone := make(chan error, 1)
	go func() {
		_, err := service.CreateArc(context.Background(), "Two")
		secondDone <- err
	}()
	readDone := make(chan error, 1)
	go func() {
		_, err := service.Outline(context.Background())
		readDone <- err
	}()

	select {
	case err := <-secondDone:
		t.Fatalf("second mutation completed before first was released: %v", err)
	case err := <-readDone:
		t.Fatalf("outline read completed during a partial mutation: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(releaseFirstLoad)
	if err := <-firstDone; err != nil {
		t.Fatalf("first CreateArc() error = %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second CreateArc() error = %v", err)
	}
	if err := <-readDone; err != nil {
		t.Fatalf("Outline() error = %v", err)
	}
	if files.loadCalls != 5 {
		t.Fatalf("load calls = %d, want 5 serialized loads and post-write reloads", files.loadCalls)
	}
}
