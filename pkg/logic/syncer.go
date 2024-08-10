package logic

import (
	"context"
	"fmt"
	"time"
)

type Syncer struct {
	stopCh    chan struct{}
	syncReqCh chan *syncRequest
	interval  time.Duration
}

type syncRequest struct {
	fn       func() error
	deadline time.Time
	cancel   context.CancelFunc
}

func (s *Syncer) Start() {
	reqs := make([]*syncRequest, 0)
	timer := time.NewTimer(s.interval)
LOOP:
	for {
		select {
		case <-s.stopCh:
			for _, req := range reqs {
				if req.deadline.Before(time.Now()) {
					continue
				}

				if err := req.fn(); err != nil {
					fmt.Println("err: ", err)
				}
			}
			break LOOP
		case req := <-s.syncReqCh:
			reqs = append(reqs, req)
		case <-timer.C:
			newReqs := make([]*syncRequest, 0)
			for _, req := range reqs {
				if req.deadline.Before(time.Now()) {
					if req.cancel != nil {
						req.cancel()
					}
					continue
				}
				err := req.fn()
				if err != nil {
					newReqs = append(newReqs, req)
				} else {
					if req.cancel != nil {
						req.cancel()
					}
				}
			}
			timer.Reset(s.interval)
		}
	}
}
