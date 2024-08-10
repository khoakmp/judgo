package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/khoakmp/judgo/pkg/base"
	"github.com/redis/go-redis/v9"
)

type RDB struct {
	client *redis.Client
}

type SubmissionDescription struct {
	Id         string `json:"id"`
	SourceCode string `json:"src"`
	Username   string `json:"username"`
	Language   string `json:"language"`
	ProblemId  string `json:"problem_id"`
	ContestId  string `json:"contest_id"`
	InContest  bool   `json:"in_contest"`
	//CompilerID int    `json:"compiler_id"`
}

// need contestId -> update , incontest, retried and maxretried

const appPrefix = "judgo:"

const (
	submissionKeyPrefix     = appPrefix + "s:"
	submissionResultPrefix  = appPrefix + "s:results:"  // use with hash: subtestid -> encoded result
	judgeTaskKeyPrefix      = appPrefix + "judge:task:" //  with submission id
	practicePendingQueueKey = appPrefix + "practice:pending:q"
	contestPendingQueueKey  = appPrefix + "contest:pending:q"
	leaseQueueKey           = appPrefix + "lease:q"
)

const enqueueCmd = `
	redis.call("SET", KEYS[1] .. ARGV[1], ARGV[2])
	redis.call("SET", KEYS[2] .. ARGV[1], ARGV[3])
	redis.call("RPUSH", KEYS[3] .. ARGV[1], ARGV[1])

	for i=4,#ARGV,2 do 
		redis.call("HSET", KEYS[4], ARGV[i], ARGV[i+1])
	end
	return "OK"	
`

func (r *RDB) Enqueue(ctx context.Context, t *base.JudgeSubmissionTask) error {
	submission := t.SubmissionDescription.Encode()
	taskDescription := t.JudgeTaskDescription.Encode()

	keys := []string{
		submissionKeyPrefix,
		judgeTaskKeyPrefix,
		submissionResultPrefix,
	}

	if t.SubmissionDescription.InContest {
		keys = append(keys, contestPendingQueueKey)
	} else {
		keys = append(keys, practicePendingQueueKey)
	}
	args := []interface{}{
		t.Id,
		submission,
		taskDescription,
	}
	for id, res := range t.Results {
		encoded := res.Encode()
		args = append(args, id, encoded)
	}
	return r.client.Eval(ctx, enqueueCmd, keys, args...).Err()
}

// rpush lpop dung
const pickOneSubmissionCmd = `
 	local id = redis.call("LPOP", KEYS[1])	
	if id then
		local results = {}
		local sub_key = KEYS[2] .. id
		local submission = redis.call("GET", sub)
		table.insert(results, submission)
		
		local t = redis.call("GET", KEYS[3] .. id )
		table.insert(results, t)
	
		local subtests = redis.call("HGETALL", KEYS[4] .. id)
		table.insert(results, subtests)
		redis.call("ZADD", KEYS[5], ARGV[1], id)
		return results
	end

	return nil
	`

var ErrQueueEmpty = errors.New("queue is empty")

func (r *RDB) PickOneSubmission() (*base.JudgeSubmissionTask, *time.Time, error) {
	queues := []string{contestPendingQueueKey, practicePendingQueueKey}
	for _, q := range queues {
		keys := []string{
			q, submissionKeyPrefix, judgeTaskKeyPrefix, submissionResultPrefix,
		}
		leaseDeadline := time.Now().Add(base.DefaultLeaseDuration)

		result, err := r.client.Eval(context.Background(), pickOneSubmissionCmd, keys,
			leaseDeadline.UnixMilli()).Result()

		if err != nil {
			if err == redis.Nil {
				continue
			}
			return nil, nil, err
		}
		arr := result.([]interface{})
		t := new(base.JudgeSubmissionTask)
		t.JudgeTaskDescription.Decode([]byte(arr[1].(string)))

		t.SubmissionDescription = new(base.SubmissionDescription)
		t.SubmissionDescription.Decode([]byte(arr[0].(string)))

		//json.Unmarshal([]byte(arr[0].(string)), t.Submission)
		//json.Unmarshal([]byte(arr[1].(string)), t)

		subtests := arr[1].([]interface{})
		for i := 0; i < len(subtests)/2; i++ {
			id, _ := strconv.Atoi(subtests[i<<1].(string))
			subtestResult := new(base.SubtestResult)
			json.Unmarshal([]byte(subtests[i<<1|1].(string)), subtestResult)
			t.Results[id] = subtestResult
		}

		t.Lease = base.NewLease(leaseDeadline)
		t.Mutex = &sync.Mutex{}

		return t, &leaseDeadline, nil
	}
	return nil, nil, ErrQueueEmpty
}

type UpdateSubtestResultParam struct {
	SubmissionId string
	SubtestId    int
	result       *base.SubtestResult
}

func (r *RDB) UpdateSubtestResult(ctx context.Context, param UpdateSubtestResultParam) error {
	encoded, _ := json.Marshal(param.result)
	return r.client.HSet(ctx, fmt.Sprintf("%s%s", submissionResultPrefix, param.SubmissionId), param.SubtestId, encoded).Err()
}

func (r *RDB) UpdatePartialResult(t *base.JudgeSubmissionTask, subtestID int) error {
	encoded := t.Results[subtestID].Encode()
	key := fmt.Sprintf("%s%s", submissionResultPrefix, t.Id)
	return r.client.HSet(context.Background(), key, subtestID, encoded).Err()
}

/* type SubmissionTotalResult struct {
	SubtestResult
	TotalPoint int `json:"total_point"`
} */

const markJudgeSubmissionCompleteCmd = `
	redis.call("ZREM", KEYS[1], ARGV[1])
	return "OK"
`

func (r *RDB) MarkJudgeSubmissionComplete(ctx context.Context, submissionId string) error {
	keys := []string{
		leaseQueueKey,
	}
	args := []interface{}{submissionId}
	return r.client.Eval(ctx, markJudgeSubmissionCompleteCmd, keys, args...).Err()
}

const completeJudgeSubmissionTaskCmd = `
	-- set task result 
	redis.call("SET" ,KEYS[1], ARGV[1])
	-- delete submission description
	redis.call("DEL", KEYS[2]) 
	-- remove submission id from lease queue
	redis.call("ZREM", KEYS[3], ARGV[2])
	return "OK"
`

func (r *RDB) CompleteJudgeSubmissionTask(ctx context.Context, t *base.JudgeSubmissionTask) error {

	taskEncoded := t.Encode()
	keys := []string{
		fmt.Sprintf("%s%s", judgeTaskKeyPrefix, t.Id),
		fmt.Sprintf("%s%s", submissionKeyPrefix, t.Id),
		leaseQueueKey,
	}
	args := []interface{}{
		taskEncoded,
		t.Id,
	}
	return r.client.Eval(ctx, completeJudgeSubmissionTaskCmd, keys, args...).Err()
}

const extendLeaseCmd = `
	for i=2,#ARGV,1 do 
		redis.call("ZADD", KEYS[1], "XX", ARGV[1], ARGV[2])
	end 
	return "OK"
`

func (r *RDB) ExtendLease(ids []string, deadline time.Time) error {
	keys := []string{
		leaseQueueKey,
	}
	args := []interface{}{
		deadline.UnixMilli(),
	}
	for _, id := range ids {
		args = append(args, id)
	}
	return r.client.Eval(context.Background(), extendLeaseCmd, keys, args...).Err()
}
