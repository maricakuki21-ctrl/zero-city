package workbench

import "testing"

func TestRecovery_PostCommitNeverRedispatchesSameRequest(t *testing.T) {
	t.Parallel()

	for _, recovery := range []RecoveryContext{
		{Phase: RecoveryPostCommit, CanonicalRequestID: "req_1"},
		{Phase: RecoveryTaskAccepted, MediaTaskID: "task_1"},
	} {
		action, err := SelectRecoveryAction(recovery)
		if err != nil {
			t.Fatalf("SelectRecoveryAction(%#v) error = %v", recovery, err)
		}
		if action == RecoveryRedispatchSameRequest {
			t.Fatalf("SelectRecoveryAction(%#v) = %q", recovery, action)
		}
	}
}

func TestRecovery_PreCommitMayRedispatchSameRequest(t *testing.T) {
	t.Parallel()

	action, err := SelectRecoveryAction(RecoveryContext{Phase: RecoveryPreCommit})
	if err != nil || action != RecoveryRedispatchSameRequest {
		t.Fatalf("SelectRecoveryAction() = %q, %v", action, err)
	}
}
