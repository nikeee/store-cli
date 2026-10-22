package google

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/api/androidpublisher/v3"
	"google.golang.org/api/googleapi"
)

// ErrCommitUnknown is in the error of a commit that got no answer from Play
// (a transport error): the edit may have been committed.
var ErrCommitUnknown = errors.New("the commit outcome is unknown, check Play Console")

// CommitOptions configure how withEdit ends an edit.
type CommitOptions struct {
	// ChangesNotSentForReview commits with changesNotSentForReview=true, for
	// apps whose changes must be sent for review manually in Play Console.
	ChangesNotSentForReview bool
	// DryRun deletes the edit instead of committing it (--dry-run).
	DryRun bool
}

// withEdit runs fn in a new edit of the app; fn gets the edit ID. Contract:
// when fn succeeds the edit is committed. When fn or the commit fails, the
// edit is deleted, so nothing is published, and that error is returned; a
// commit without an answer from Play may have gone through, so its error
// wraps ErrCommitUnknown. With
// opts.DryRun fn still runs and must itself skip uploads and every other
// mutating call; the edit is then deleted instead of committed. The delete
// runs even when ctx is cancelled; a failed delete is only a warning, since
// an uncommitted edit is never published.
func (s *Service) withEdit(ctx context.Context, opts CommitOptions, fn func(editID string) error) error {
	edit, err := s.api.Edits.Insert(s.packageName, &androidpublisher.AppEdit{}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("create edit: %w", err)
	}
	if err := fn(edit.Id); err != nil {
		s.deleteEdit(ctx, edit.Id)
		return err
	}
	if opts.DryRun {
		s.deleteEdit(ctx, edit.Id)
		return nil
	}
	commit := s.api.Edits.Commit(s.packageName, edit.Id).Context(ctx)
	if opts.ChangesNotSentForReview {
		commit = commit.ChangesNotSentForReview(true)
	}
	if _, err := commit.Do(); err != nil {
		s.deleteEdit(ctx, edit.Id)
		var ge *googleapi.Error
		if !errors.As(err, &ge) {
			return fmt.Errorf("commit edit: %w (%w)", err, ErrCommitUnknown)
		}
		return fmt.Errorf("commit edit: %w", err)
	}
	return nil
}

// deleteEditTimeout bounds the cleanup delete, which ignores the caller's
// cancellation.
const deleteEditTimeout = 30 * time.Second

func (s *Service) deleteEdit(ctx context.Context, editID string) {
	// After Ctrl-C or a timeout, ctx is done but the edit must still go.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), deleteEditTimeout)
	defer cancel()
	if err := s.api.Edits.Delete(s.packageName, editID).Context(ctx).Do(); err != nil {
		s.report.Warnf("could not delete edit %s: %v", editID, FromAPIError(err))
	}
}

// CheckAuth verifies the credentials and the access to the app by creating
// and deleting an edit.
func (s *Service) CheckAuth(ctx context.Context) error {
	return s.withEdit(ctx, CommitOptions{DryRun: true}, func(string) error { return nil })
}
