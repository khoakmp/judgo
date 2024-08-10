package logic

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/gammazero/workerpool"
	"github.com/khoakmp/judgo/pkg/base"
)

type Complier struct {
	wp           *workerpool.WorkerPool
	judger       *Judger
	compileErrCh chan *compileResult
}

const srcDirPath = "../src/"
const binDirPath = "../bin/"

func compileCommand(binfileName, srcfile string) []string {
	return []string{"g++", "-o", binfileName, srcfile}
}

func (c *Complier) doCompile(s *base.SubmissionDescription) (binfileName string, err error) {
	srcFilename := fmt.Sprintf("%s%s.%s", srcDirPath, s.Id, s.Language)
	_, err = os.Create(srcFilename)
	if err != nil {
		return
	}
	binfilename := fmt.Sprintf("%s%s", binDirPath, s.Id)
	cmdArr := compileCommand(binfilename, srcFilename)
	cmd := exec.Command(cmdArr[0], cmdArr[1:]...)
	err = cmd.Run()
	if err != nil {
		return
	}
	return
}
