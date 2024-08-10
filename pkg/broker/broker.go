package broker

import (
	"context"
	"time"

	"github.com/khoakmp/judgo/pkg/base"
)

type Broker interface {
	PickOneSubmission() (*base.JudgeSubmissionTask, *time.Time, error)
	CompleteJudgeSubmissionTask(ctx context.Context, t *base.JudgeSubmissionTask) error
	UpdatePartialResult(t *base.JudgeSubmissionTask, subtestID int) error
	ExtendLease(ids []string, deadline time.Time) error
	Enqueue(t *base.JudgeSubmissionTask) error
}
