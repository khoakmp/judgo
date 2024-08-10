package storage

import (
	"context"

	"github.com/khoakmp/judgo/pkg/base"
)

type Store interface {
	UpdateSubmissionResult(ctx context.Context, t *base.JudgeSubmissionTask) error
}
