package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	//	"bytes"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	//	"6.5840/labgob"
	"6.5840/labrpc"
)

// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in part 2D you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh, but set CommandValid to false for these
// other uses.
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int

	// For 2D:
	SnapshotValid bool
	Snapshot      []byte
	SnapshotTerm  int
	SnapshotIndex int
}

// A Go object implementing a single Raft peer.
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.

	// Persistent state on all servers
	currentTerm int // initialized to 0 on first boot, increase monotonically
	voteFor     int // candidateId that received vote in current term (or nil(-1) if none)
	log         []Entry

	// Voltile state on all servers
	commitIndex int // index of the highest log entry known to be committed (initialized to 0, increases monotonically)
	lastApplied int // index of the highest log entry applied to state machine (initialized to 0, increases monotonically)

	// Volatile state on all leaders
	// Reinitialized after election
	nextIndex  []int // for each server, index of the next log entry to send to that server (initialized to leader last log index + 1)
	matchIndex []int // for each server, index of the highest log entry known to be replicated on server (initialized to 0, increases monotonically)

	// Supplementary information
	role                            // role of a raft peer
	lastResetElectionTime time.Time // wait a period of time to receive tpc call, refresh once is called
	applyCh               chan ApplyMsg
	gotVotes              int // The count of votes received in an election
}

type Entry struct {
	Command interface{}
	Term    int
}

// type of peer's role
type role int

const (
	Leader = iota
	Candidate
	Follower
)

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	var term int
	var isleader bool

	rf.mu.Lock()
	defer rf.mu.Unlock()
	// Your code here (2A).
	term = rf.currentTerm
	isleader = rf.role == Leader
	return term, isleader
}

// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
// before you've implemented snapshots, you should pass nil as the
// second argument to persister.Save().
// after you've implemented snapshots, pass the current snapshot
// (or nil if there's not yet a snapshot).
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// raftstate := w.Bytes()
	// rf.persister.Save(raftstate, nil)
}

// restore previously persisted state.
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
}

// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (2D).

}

type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term         int // candidate's term
	CandidateId  int // candidate requesting vote
	LastLogIndex int // index of candidate's last log entry
	LastLogTerm  int // term of candidate's last log entry
}

type RequestVoteReply struct {
	// Your data here (2A).
	Term        int  // cuurrentTerm, for candidate to update itself
	VoteGranted bool // true means candidate received vote
}

// example RequestVote RPC handler.
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// Rules for all servers
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.role = Follower
		rf.voteFor = -1
	}

	reply.Term = rf.currentTerm
	reply.VoteGranted = false

	// Receiver implemention
	if args.Term < rf.currentTerm {
		return
	}
	if (rf.voteFor == -1 || rf.voteFor == args.CandidateId) && rf.isAtLeastUptoDate(args) {
		rf.voteFor = args.CandidateId
		reply.VoteGranted = true
		rf.lastResetElectionTime = time.Now()
		DPrintf("peer: %d  role: %d  term: %d ===> give one vote to peer %d", rf.me, rf.role, rf.currentTerm, args.CandidateId)
		return
	}
}

// true, if candidate's log is at least as up-to-date as receiver's log
// 候选者的日志至少和接受者一样新，>=
// Raft determines which of two logs is more up-to-date by comparing term and index of the last entries in the logs.
// if the logs have last entries with different terms, then the log with later term is more up-to-date
// if the logs end with the same term, then whicher log is longer is more up-to-date.
func (rf *Raft) isAtLeastUptoDate(c *RequestVoteArgs) bool {
	last := len(rf.log) - 1
	term := rf.log[last].Term

	if c.LastLogTerm > term {
		return true
	}
	if c.LastLogTerm == term && c.LastLogIndex >= last {
		return true
	}
	return false
}

// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true

	// Your code here (2B).

	return index, term, isLeader
}

// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

func (rf *Raft) ticker() {
	for rf.killed() == false {

		// Your code here (2A)
		// Check if a leader election should be started.
		sleepTime := time.Now()
		ms := 50 + (rand.Int63() % 300) // pause for a random amount of time between 50 and 350 milliseconds.
		//ms := rand.Intn(150) + 150
		time.Sleep(time.Duration(ms) * time.Millisecond)

		rf.mu.Lock()
		switch rf.role {
		case Leader: // Leader doesn't need to elctec, does nothing

		case Candidate, Follower: // For candidate election time out, for follower may it's time start an election
			if rf.lastResetElectionTime.Before(sleepTime) {
				go rf.startElection()
			}
		}
		rf.mu.Unlock()
	}
}

func (rf *Raft) startElection() {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	rf.role = Candidate
	rf.currentTerm++
	rf.voteFor = rf.me
	rf.gotVotes = 1
	rf.lastResetElectionTime = time.Now()
	DPrintf("peer: %d  role: %d  term: %d ===> start to election", rf.me, rf.role, rf.currentTerm)

	for peer := range rf.peers {
		if peer != rf.me {
			go rf.requestVoteFrom(peer, rf.currentTerm)
		}
	}
}

func (rf *Raft) requestVoteFrom(peer, votedTerm int) {
	// Init RequestVoteArgs
	rf.mu.Lock()
	args := &RequestVoteArgs{
		Term:         rf.currentTerm,
		CandidateId:  rf.me,
		LastLogIndex: len(rf.log) - 1,
		LastLogTerm:  rf.log[len(rf.log)-1].Term,
	}
	reply := &RequestVoteReply{}
	rf.mu.Unlock()

	ok := rf.sendRequestVote(peer, args, reply)
	if !ok {
		return
	}

	// Processing response
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.role != Candidate {
		return
	}
	// Rules for all servers
	if rf.currentTerm < reply.Term {
		rf.currentTerm = reply.Term
		rf.role = Follower
		rf.voteFor = -1
		return
	}

	// Simply ignore expired replies
	if reply.Term != rf.currentTerm || votedTerm != rf.currentTerm {
		return
	}

	if reply.VoteGranted {
		rf.gotVotes++
	}
	if rf.gotVotes > len(rf.peers)/2 {
		rf.role = Leader
		rf.voteFor = -1
		DPrintf("peer: %d  role: %d  term: %d ===> Sufficient votes, been Leader", rf.me, rf.role, rf.currentTerm)
		// Reinitialized after election
		for i := range rf.peers {
			rf.nextIndex[i] = len(rf.log)
			rf.matchIndex[i] = 0
		}
		go rf.heartBeatTicker()
	}
}

func (rf *Raft) heartBeatTicker() {
	heartTicker := time.Tick(time.Duration(20) * time.Millisecond)
	appendTicker := time.Tick(time.Duration(10) * time.Millisecond)

	// send heartbeat immediately once wins an election
	// here the code written is just because ticker will not trigger immediately
	rf.atomicRangeSend()

	for !rf.killed() {
		if _, isLeader := rf.GetState(); !isLeader {
			return
		}

		select {
		case <-heartTicker:
			rf.atomicRangeSend()
		case <-appendTicker:
			// 2B暂时不实现
		}
	}
}

func (rf *Raft) atomicRangeSend() {
	rf.mu.Lock()
	for peer := range rf.peers {
		if rf.role != Leader {
			rf.mu.Unlock()
			return
		}
		if peer != rf.me {
			go rf.heartBeatTo(peer, true)
		}
	}
	rf.mu.Unlock()
}

// Periodic hearbeats(AppendEntries RPCs that carry no log entries) that leader sent
func (rf *Raft) heartBeatTo(peer int, empty bool) {
	rf.mu.Lock()
	DPrintf("peer: %d  role: %d  term: %d ===> send heartbear to %d", rf.me, rf.role, rf.currentTerm, peer)
	var entries []Entry
	var plIndex, plTerm int
	if !empty {
		plIndex = rf.nextIndex[peer] - 1
		plTerm = rf.log[plIndex].Term
		entries = make([]Entry, len(rf.log[plIndex+1:]))
		copy(entries, rf.log[plIndex+1:])
	}
	args := &AppendEntriesArgs{
		Term:         rf.currentTerm,
		LeaderId:     rf.me,
		PrevLogIndex: plIndex,
		PrevLogTerm:  plTerm,
		Entries:      entries,
		LeaderCommit: rf.commitIndex,
	}
	reply := &AppendEntriesReply{}
	rf.mu.Unlock()

	ok := rf.sendAppendEntries(peer, args, reply)
	if !ok {
		return
	}

	rf.mu.Lock()
	defer rf.mu.Unlock()

	// Rules for all servers
	if rf.currentTerm < reply.Term {
		rf.currentTerm = reply.Term
		rf.role = Follower
		rf.voteFor = -1
	}

}

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// Rules on all servers
	if rf.currentTerm < args.Term {
		rf.currentTerm = args.Term
		rf.role = Follower
		rf.voteFor = -1
	}
	if rf.currentTerm > args.Term {
		reply.Term = rf.currentTerm
		reply.Success = false
		return
	}

	// Jude appendRPC type
	if args.Entries == nil {
		reply.Term = rf.currentTerm
		reply.Success = false
		rf.lastResetElectionTime = time.Now()
		DPrintf("peer: %d  role: %d  term: %d ===> receiv hearBeat rpc from %d", rf.me, rf.role, rf.currentTerm, args.LeaderId)
		return
	}
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

type AppendEntriesArgs struct {
	Term         int
	LeaderId     int
	PrevLogIndex int
	PrevLogTerm  int
	Entries      []Entry
	LeaderCommit int
}

type AppendEntriesReply struct {
	Term    int
	Success bool

	//FollowerIndex int // Leader传来的PreLogIndex 快重传
	//FollowerTerm  int // Leader传来的PreLogTerm  快重传
}

// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here (2A, 2B, 2C).
	rf.currentTerm = 0
	rf.voteFor = -1
	rf.log = append(rf.log, Entry{nil, rf.currentTerm}) // log's first index is 1

	rf.commitIndex = 0
	rf.lastApplied = 0

	for range rf.peers {
		rf.nextIndex = append(rf.nextIndex, 1)
		rf.matchIndex = append(rf.matchIndex, 0)
	}

	rf.role = Follower
	rf.applyCh = applyCh
	rf.lastResetElectionTime = time.Now()

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	// start ticker goroutine to start elections
	go rf.ticker()

	return rf
}
