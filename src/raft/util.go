package raft

import "log"

// Debugging
const Debug = false

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug {
		log.Printf(format, a...)
	}
	return
}

func (rf *Raft) printRole() (r string) {
	switch rf.role {
	case Follower:
		r = "Follower"
	case Candidate:
		r = "Candidate"
	case Leader:
		r = "Leader"
	}
	return
}
