package server

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/khoakmp/judgo/pkg/base"
	"github.com/khoakmp/judgo/pkg/testcase"
)

func (s *Server) handleCreateSubmission(w http.ResponseWriter, r *http.Request) {
	// 1. create submission dung?
	submission := new(base.SubmissionDescription)
	reqBody, _ := io.ReadAll(r.Body)
	json.Unmarshal(reqBody, submission)
	submission.Username = "kmp"
	submission.Id = uuid.New().String()
	meta, err := s.testcase.GetTestcaseMetadata(submission.ProblemId)

	if err != nil {
		if err == testcase.ErrTestcaseNotFound {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	task := &base.JudgeTaskDescription{
		MaxRetry:     3,
		FinalVerdict: base.VerdictUnjudge,
		TimeLimit:    meta.TimeLimit,
		Memory:       meta.MemoryLimit,
	}
	results := make(map[int]*base.SubtestResult)

	for i := 0; i < meta.Quantity; i++ {
		results[i] = &base.SubtestResult{
			VerdictCode: base.VerdictUnjudge,
		}
	}

	t := &base.JudgeSubmissionTask{
		SubmissionDescription: submission,
		JudgeTaskDescription:  task,
		Results:               results,
	}

	err = s.broker.Enqueue(t)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

}
