package logic

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gammazero/workerpool"
	"github.com/khoakmp/judgo/pkg/base"
	"github.com/khoakmp/judgo/pkg/broker"
	"github.com/khoakmp/judgo/pkg/storage"
	"github.com/khoakmp/judgo/pkg/testcase"
)

type Processor struct {
	stopCh     chan struct{}
	compiler   *Complier
	wp         *workerpool.WorkerPool
	judger     *Judger
	broker     broker.Broker
	slotCh     chan struct{}
	quitCh     chan struct{}
	taskInfoCh chan *base.JudgeSubmissionTask
	doneCh     chan string
	syncReqCh  chan *syncRequest
	store      storage.Store
	testcase   testcase.TestcaseManager
}

type compileResult struct {
	task        *base.JudgeSubmissionTask
	binfilename string
	err         error
}

func (p *Processor) Start() {
LOOP:
	for {
		select {
		case <-p.stopCh:
			break LOOP

		default:
			p.exec()
		}
	}
}

func (p *Processor) exec() {
	select {
	case <-p.quitCh:
		return
	case p.slotCh <- struct{}{}:
		task, _, err := p.broker.PickOneSubmission()
		if err != nil {
			if err == broker.ErrQueueEmpty {
				time.Sleep(time.Second)
			}
		}
		var verdicted int
		for _, r := range task.Results {
			if r.VerdictCode != base.VerdictUnjudge {
				verdicted++
			}
		}
		task.Verdicted = verdicted

		p.taskInfoCh <- task
		p.process(task)
	}
}

func (p *Processor) process(t *base.JudgeSubmissionTask) {
	p.wp.Submit(func() {
		defer func() {
			<-p.slotCh
			p.doneCh <- t.Id
		}()

		binfile, err := p.compiler.doCompile(t.SubmissionDescription)
		if err != nil {
			t.FinalVerdict = base.VerdictCompileError
			t.Error = err.Error()
			err := p.complete(t)
			if err != nil {
				p.syncReqCh <- &syncRequest{
					fn: func() error {
						return p.complete(t)
					},
					deadline: t.Lease.Deadline(),
					cancel:   nil,
				}
			}
			return
		}

		var wg sync.WaitGroup
		// 1. no co the co dang acm || oi dung?

		for subtestId, result := range t.Results {
			if result.VerdictCode == base.VerdictUnjudge {
				fmt.Println("judge:", subtestId)
				wg.Add(1)
				p.judger.submit(&judgeTask{
					binfileName: binfile,
					subtestId:   subtestId,
					task:        t,
					wg:          &wg,
				})
			}
		}

		wg.Wait()
		if !t.Lease.IsValid() {
			return
		}
		if t.Type == base.TypeProblemACM {
			for _, result := range t.Results {
				if result.VerdictCode != base.VerdictAccepted {
					t.FinalVerdict = result.VerdictCode
					break
				}
				if t.ExecTime < result.ExecTime {
					t.ExecTime = result.ExecTime
				}
				if t.Memory < result.MemoryUsage {
					t.Memory = result.MemoryUsage
				}
			}
		}
		var points []int
		if t.Type == base.TypeProblemOI {
			points = p.testcase.GetTestcasePoints(t.ProblemId)
		}
		t.CalculateFinalResult(points)

		err = p.complete(t)
		if err != nil {
			p.syncReqCh <- &syncRequest{
				fn: func() error {
					return p.complete(t)
				},
				deadline: t.Lease.Deadline(),
			}
		}

	})
	// base on redis 100% cung duoc co the dung cai gi do dung
	// scale tam 20 judger + 3 for other service la ok dung?
	// van de voi cai gi dung?
	// co the duy tri cai dong do cung ok dung
	// next la ban chat cua no la sync dung?
	// dung do van de ve co bancung ok no problem dung
	// 50 judger co ve van ok dung?
	// may cai khac thi nhanh dung do van de se the na dung?

}

func (p *Processor) complete(t *base.JudgeSubmissionTask) error {
	if !t.Lease.IsValid() {
		return nil
	}

	ctx, cancel := context.WithDeadline(context.Background(), t.Lease.Deadline())
	err := p.store.UpdateSubmissionResult(ctx, t)
	if err != nil {
		cancel()
		return err
	}

	err = p.broker.CompleteJudgeSubmissionTask(ctx, t)
	if err != nil {
		p.syncReqCh <- &syncRequest{
			fn: func() error {
				return p.broker.CompleteJudgeSubmissionTask(ctx, t)
			},
			deadline: t.Lease.Deadline(),
			cancel:   cancel,
		}
		return nil
	}
	cancel()
	return nil
}
