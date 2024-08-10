package logic

import (
	"fmt"
	"time"

	"github.com/khoakmp/judgo/pkg/base"
	"github.com/khoakmp/judgo/pkg/broker"
)

type Monitor struct {
	taskMap    map[string]*base.JudgeSubmissionTask
	taskInfoCh chan *base.JudgeSubmissionTask
	stopCh     chan struct{}
	interval   time.Duration
	broker     broker.Broker
	syncReqch  chan *syncRequest
	doneCh     chan string
}

func (m *Monitor) Start() {
	timer := time.NewTimer(m.interval)
LOOP:
	for {
		select {
		case <-m.stopCh:
			break LOOP
		case task := <-m.taskInfoCh:
			m.taskMap[task.Id] = task
		case id := <-m.doneCh:
			delete(m.taskMap, id)
		case <-timer.C:
			ids := make([]string, 0)
			for id, task := range m.taskMap {
				if !task.Lease.IsValid() {
					task.Lease.NotifyExpried()
				} else {
					ids = append(ids, id)
				}
			}
			deadline := time.Now().Add(base.DefaultLeaseDuration)
			if err := m.broker.ExtendLease(ids, deadline); err != nil {
				fmt.Println("failed to extend lease,cause by:", err)
			}
			timer.Reset(m.interval)
		}
	}
}
