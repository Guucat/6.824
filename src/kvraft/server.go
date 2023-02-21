package kvraft

import (
	"6.5840/labgob"
	"6.5840/labrpc"
	"6.5840/raft"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

const Debug = false

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug {
		log.Printf(format, a...)
	}
	return
}

type Op struct {
	// Your definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
	ClientId int64
	ReqId    int64
	Kind     string // command type, Put/Append/Get
	Key      string
	Value    string
}

type KVServer struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg
	dead    int32 // set by Kill()

	maxraftstate int // snapshot if log grows this big

	// Your definitions here.
	db              map[string]string // kvDB to store value
	clientsMaxReqId map[int64]int64   // max reqId clients had receiced
	agreeChan       map[int]chan Op   // command index to Op channel
}

func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {
	// Your code here.
	command := Op{
		Kind: "Get",
		Key:  args.Key,
	}
	comIndex, _, isLeader := kv.rf.Start(command)
	reply.Err = ErrWrongLeader
	if !isLeader {
		return
	}

	ch := kv.getAgreeChan(comIndex)
	op := Op{}
	select {
	case op = <-ch:
		close(ch)
	case <-time.After(time.Duration(500) * time.Millisecond): // timeout, the client may not be leader
		reply.Err = ErrNoResponse
		return
	}

	// old leader in net partition and new leader was elected, the log may be overwrited
	if !isSameOp(command, op) {
		return
	}

	kv.mu.Lock()
	reply.Err = OK
	//if _, ok := kv.db[args.Key]; !ok {
	//	reply.Err = ErrNoKey
	//}
	reply.Value = kv.db[args.Key]
	kv.mu.Unlock()
}

func isSameOp(x, y Op) bool {
	return x.ClientId == y.ClientId &&
		x.ReqId == y.ReqId &&
		x.Kind == y.Kind &&
		x.Key == y.Key &&
		x.Value == y.Value
}

func (kv *KVServer) PutAppend(args *PutAppendArgs, reply *PutAppendReply) {
	// Your code here.
	command := Op{
		ClientId: args.CliendId,
		ReqId:    args.ReqId,
		Kind:     args.Op,
		Key:      args.Key,
		Value:    args.Value,
	}
	comIndex, _, isLeader := kv.rf.Start(command)

	reply.Err = ErrWrongLeader
	if !isLeader {
		return
	}

	ch := kv.getAgreeChan(comIndex)
	op := Op{}
	select {
	case op = <-ch:
		close(ch)
	case <-time.After(time.Duration(500) * time.Millisecond):
		reply.Err = ErrNoResponse
		return
	}

	if !isSameOp(command, op) {
		return
	}

	reply.Err = OK
}

func (kv *KVServer) waitAgree() {
	for !kv.killed() {
		msg := <-kv.applyCh
		op := msg.Command.(Op) // the consensual command

		kv.mu.Lock()
		maxReqId, ok := kv.clientsMaxReqId[op.ClientId]
		if !ok || op.ReqId > maxReqId { // leader only handle fresh request
			switch op.Kind {
			case "Put":
				kv.db[op.Key] = op.Value
			case "Append":
				kv.db[op.Key] += op.Value
			}
			kv.clientsMaxReqId[op.ClientId] = maxReqId
		}
		kv.mu.Unlock()

		kv.getAgreeChan(msg.CommandIndex) <- op
	}
}

func (kv *KVServer) getAgreeChan(commandIndex int) chan Op {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	ch, ok := kv.agreeChan[commandIndex]
	if !ok {
		ch = make(chan Op, 1) // can't block the chan
		kv.agreeChan[commandIndex] = ch
	}
	return ch
}

// the tester calls Kill() when a KVServer instance won't
// be needed again. for your convenience, we supply
// code to set rf.dead (without needing a lock),
// and a killed() method to test rf.dead in
// long-running loops. you can also add your own
// code to Kill(). you're not required to do anything
// about this, but it may be convenient (for example)
// to suppress debug output from a Kill()ed instance.
func (kv *KVServer) Kill() {
	atomic.StoreInt32(&kv.dead, 1)
	kv.rf.Kill()
	// Your code here, if desired.
}

func (kv *KVServer) killed() bool {
	z := atomic.LoadInt32(&kv.dead)
	return z == 1
}

// servers[] contains the ports of the set of
// servers that will cooperate via Raft to
// form the fault-tolerant key/value service.
// me is the index of the current server in servers[].
// the k/v server should store snapshots through the underlying Raft
// implementation, which should call persister.SaveStateAndSnapshot() to
// atomically save the Raft state along with the snapshot.
// the k/v server should snapshot when Raft's saved state exceeds maxraftstate bytes,
// in order to allow Raft to garbage-collect its log. if maxraftstate is -1,
// you don't need to snapshot.
// StartKVServer() must return quickly, so it should start goroutines
// for any long-running work.
func StartKVServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister, maxraftstate int) *KVServer {
	// call labgob.Register on structures you want
	// Go's RPC library to marshall/unmarshall.
	labgob.Register(Op{})

	kv := new(KVServer)
	kv.me = me
	kv.maxraftstate = maxraftstate

	// You may need initialization code here.

	kv.applyCh = make(chan raft.ApplyMsg)
	kv.rf = raft.Make(servers, me, persister, kv.applyCh)

	kv.db = make(map[string]string)
	kv.clientsMaxReqId = make(map[int64]int64)
	kv.agreeChan = make(map[int]chan Op)
	go kv.waitAgree()
	return kv
}
