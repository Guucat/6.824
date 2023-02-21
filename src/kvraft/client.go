package kvraft

import (
	"6.5840/labrpc"
	"crypto/rand"
	"math/big"
	"sync/atomic"
)

type Clerk struct {
	servers []*labrpc.ClientEnd
	// You will have to modify this struct.
	leaderId int
	clientId int64
	reqId    int64 // latest request sequence number
}

func nrand() int64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Int64()
	return x
}

func MakeClerk(servers []*labrpc.ClientEnd) *Clerk {
	ck := new(Clerk)
	ck.servers = servers
	// You'll have to add code here.
	ck.leaderId = 0 // try at 0
	ck.clientId = nrand()
	ck.reqId = 0 // A unique id with an increasing trend
	return ck
}

func (ck *Clerk) changeLeader() {
	ck.leaderId = (ck.leaderId + 1) % len(ck.servers)
}

// fetch the current value for a key.
// returns "" if the key does not exist.
// keeps trying forever in the face of all other errors.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("KVServer.Get", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
func (ck *Clerk) Get(key string) string {

	// You will have to modify this function.
	for {
		args := GetArgs{
			Key: key,
		}
		reply := GetReply{}

		DPrintf("client %d get %s from leader %d", ck.clientId, key, ck.leaderId)
		ok := ck.servers[ck.leaderId].Call("KVServer.Get", &args, &reply)

		if !ok || reply.Err == ErrWrongLeader {
			ck.changeLeader()
			continue
		}
		if reply.Err == ErrNoResponse {
			continue
		}
		//if reply.Err == ErrNoKey {
		//	return ""
		//}
		return reply.Value
	}
}

// shared by Put and Append.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("KVServer.PutAppend", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
func (ck *Clerk) PutAppend(key string, value string, op string) {
	// You will have to modify this function.
	reqId := atomic.AddInt64(&ck.reqId, 1)
	for {
		args := PutAppendArgs{
			Key:      key,
			Value:    value,
			Op:       op,
			CliendId: ck.clientId,
			ReqId:    reqId,
		}
		reply := PutAppendReply{}

		DPrintf("client %d %s %s val: %s from leader %d", ck.clientId, op, key, value, ck.leaderId)
		ok := ck.servers[ck.leaderId].Call("KVServer.PutAppend", &args, &reply)

		if !ok || reply.Err == ErrWrongLeader {
			ck.changeLeader()
			continue
		}
		if reply.Err == ErrNoResponse {
			continue
		}
		return
	}
}

func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, "Put")
}
func (ck *Clerk) Append(key string, value string) {
	ck.PutAppend(key, value, "Append")
}
