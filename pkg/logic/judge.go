package logic

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/gammazero/workerpool"
	"github.com/khoakmp/judgo/pkg/base"
	"github.com/khoakmp/judgo/pkg/broker"
	"github.com/khoakmp/judgo/pkg/testcase"
)

type Judger struct {
	wp       *workerpool.WorkerPool
	testcase testcase.TestcaseManager
	broker   broker.Broker
}

type judgeTask struct {
	binfileName string
	subtestId   int
	task        *base.JudgeSubmissionTask
	wg          *sync.WaitGroup
}

type judgeResult struct {
	submissionID string
	subtestID    int
	*base.SubtestResult
}

func (j *Judger) submit(t *judgeTask) {
	j.wp.Submit(func() {
		defer t.wg.Done()
		j.judge(t)
	})
}

func (j *Judger) judge(t *judgeTask) {
	inpBuf, answerBuf, err := j.testcase.GetTestcase(t.task.ProblemId, t.subtestId)
	if err != nil {
		t.task.Results[t.subtestId].ErrMsg = err.Error()
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(t.task.TimeLimit+1)*time.Millisecond)

	resultCh := make(chan *base.SubtestResult)

	outBuf := bytes.NewBuffer(nil)

	cmd := exec.CommandContext(ctx, t.binfileName)
	cmd.Stdin = bytes.NewReader(inpBuf)
	cmd.Stdout = outBuf

	go func() {
		err := cmd.Run()
		var result base.SubtestResult
		if err != nil {
			result.VerdictCode = base.VerdictRunTimeError
			result.ErrMsg = err.Error()
		} else {
			ok := checkOutput(outBuf.Bytes(), answerBuf)
			if ok {
				if cmd.ProcessState.UserTime().Milliseconds() > int64(t.task.TimeLimit) {
					result.VerdictCode = base.VerdictTimeLimitExceed
				} else if cmd.ProcessState.SysUsage().(*syscall.Rusage).Maxrss > int64(t.task.MemoryLimit) {
					result.VerdictCode = base.VerdictMemoryLimitExceed
				} else {
					result.VerdictCode = base.VerdictAccepted
					result.ExecTime = int(cmd.ProcessState.UserTime())
					result.MemoryUsage = int(cmd.ProcessState.SysUsage().(*syscall.Rusage).Maxrss)
				}
			} else {
				result.VerdictCode = base.VerdictWrongAnwser
			}
		}
		resultCh <- &result
	}()

	select {
	case result := <-resultCh:
		t.task.UpdateSubtestResult(t.subtestId, result)
		err := j.broker.UpdatePartialResult(t.task, t.subtestId)
		if err != nil {

		}
	case <-t.task.Lease.Done():
		fmt.Println("lease expried, abort subtest", t.subtestId)
	}
	cancel()
}

type BufferReader struct {
	buf []byte
}

func isDelim(c byte) bool {
	return c == ' ' || c == '\n' || c == '\t'
}
func (r *BufferReader) ReadNext() []byte {
	if len(r.buf) == 0 {
		return nil
	}
	for len(r.buf) > 0 && isDelim(r.buf[0]) {
		r.buf = r.buf[1:]
	}
	p := 0
	for p < len(r.buf) && !isDelim(r.buf[p]) {
		p++
	}
	ans := r.buf[:p]
	r.buf = r.buf[p:]
	return ans
}
func NewBufferReader(buf []byte) *BufferReader {
	return &BufferReader{
		buf: buf,
	}
}
func checkOutput(output []byte, answer []byte) bool {
	outReader := NewBufferReader(output)
	ansReader := NewBufferReader(answer)
	for {
		ans := ansReader.ReadNext()
		if ans == nil {
			if outReader.ReadNext() != nil {
				return false
			}
			return true
		}
		out := outReader.ReadNext()
		if !bytes.Equal(ans, out) {
			return false
		}
	}
}
