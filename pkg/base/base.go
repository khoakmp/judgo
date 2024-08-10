package base

import (
	"encoding/json"
	"sync"
	"time"
)

const (
	VerdictUnjudge           = 0
	VerdictCompileError      = 1
	VerdictAccepted          = 2
	VerdictWrongAnwser       = 3
	VerdictTimeLimitExceed   = 4
	VerdictMemoryLimitExceed = 5
	VerdictRunTimeError      = 6
	VerdictPartial           = 7
)

const (
	TypeProblemACM = 0
	TypeProblemOI  = 1
)

const DefaultLeaseDuration = time.Second * 30

type Lease struct {
	doneCh   chan struct{}
	expireAt time.Time
	mu       sync.Mutex
	once     sync.Once
}

func NewLease(expiration time.Time) *Lease {
	return &Lease{
		doneCh:   make(chan struct{}),
		expireAt: expiration,
		mu:       sync.Mutex{},
		once:     sync.Once{},
	}
}
func (l *Lease) IsValid() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := time.Now()
	return l.expireAt.After(n) || l.expireAt.Equal(n)
}

func (l *Lease) Reset(deadline time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.expireAt = deadline
}

func (l *Lease) Deadline() time.Time {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.expireAt
}

func (l *Lease) Done() <-chan struct{} {
	return l.doneCh
}

func (l *Lease) NotifyExpried() {
	l.once.Do(func() {
		close(l.doneCh)
	})
}

// should adding here dung/
type SubmissionDescription struct {
	Id         string `json:"id"`
	SourceCode string `json:"src"`
	Username   string `json:"username"`
	Language   string `json:"language"`
	ProblemId  string `json:"problem_id"`
	ContestId  string `json:"contest_id"`
	InContest  bool   `json:"in_contest"`
	Type       int    `json:"type"`
	//CompilerID int    `json:"compiler_id"`
}

func (s *SubmissionDescription) Encode() []byte {
	buf, _ := json.Marshal(s)
	return buf
}
func (s *SubmissionDescription) Decode(buf []byte) {
	json.Unmarshal(buf, s)
}

type JudgeSubmissionTask struct {
	*SubmissionDescription `json:"-"`
	Results                map[int]*SubtestResult `json:"-"`
	*JudgeTaskDescription
	Mutex *sync.Mutex `json:"-"`
	Lease *Lease      `json:"-"`
}

func (t *JudgeSubmissionTask) CalculateFinalResult(points []int) {
	if t.Type == TypeProblemACM {
		for _, subtestResult := range t.Results {
			if subtestResult.VerdictCode != VerdictAccepted {
				t.FinalVerdict = subtestResult.VerdictCode
			}
		}
		if t.FinalVerdict == VerdictAccepted {
			for _, subtestResult := range t.Results {
				if t.ExecTime < subtestResult.ExecTime {
					t.ExecTime = subtestResult.ExecTime
				}
				if t.Memory < subtestResult.MemoryUsage {
					t.Memory = subtestResult.MemoryUsage
				}
			}
		}
		return
	}
	accepted := 0
	for idx, subtestResult := range t.Results {
		if subtestResult.VerdictCode == VerdictAccepted {
			t.TotalPoint += points[idx]
			accepted++
		} else {
			t.FinalVerdict = subtestResult.VerdictCode
		}
	}
	if accepted > 0 {
		if accepted < len(t.Results) {
			t.FinalVerdict = VerdictPartial
		} else {
			t.FinalVerdict = VerdictAccepted
		}
	}
}

type JudgeTaskDescription struct {
	Retried      int    `json:"retried"`
	MaxRetry     int    `json:"max_retry"`
	FinalVerdict int    `json:"final_verdict"`
	TotalPoint   int    `json:"total_point"`
	ExecTime     int    `json:"exec_time"`
	Memory       int    `json:"memory"`
	Verdicted    int    `json:"verdicted"`
	Error        string `json:"error"`
	TimeLimit    int    `json:"time_limit"`
	MemoryLimit  int    `json:"mem_limit"`
}

func (t *JudgeTaskDescription) Decode(buf []byte) {
	json.Unmarshal(buf, t)
}
func (t *JudgeTaskDescription) Encode() []byte {
	buf, _ := json.Marshal(t)
	return buf
}
func (t *JudgeSubmissionTask) Encode() []byte {
	buf, _ := json.Marshal(t)
	return buf
}
func (t *JudgeSubmissionTask) Decode(buf []byte) {
	json.Unmarshal(buf, t)
}

func (t *JudgeSubmissionTask) UpdateSubtestResult(subtestID int, result *SubtestResult) {
	t.Mutex.Lock()
	defer t.Mutex.Unlock()
	t.Results[subtestID] = result
}

type SubtestResult struct {
	VerdictCode int    `json:"verdict_code"`
	ExecTime    int    `json:"exec_time"`
	MemoryUsage int    `json:"memory"`
	ErrMsg      string `json:"err_msg"`
}

func (r *SubtestResult) Encode() []byte {
	buf, _ := json.Marshal(r)
	return buf
}
