MIT6.824分布式系统课程实验的个人实现代码，课程号现已更名为[6.5840](https://pdos.csail.mit.edu/6.824/index.html)

整个实验分为两部分，第一部分是lab1，实现`MapReduce并行计算系统`来统计词频，第二部分是lab2 - lab4，实现raft共识算法，并在此基础上实现一个`分布式KV存储系统`。
# lab2 Raft
参考：[raft论文](https://pdos.csail.mit.edu/6.824/papers/raft-extended.pdf)， [guide](https://thesquareplanet.com/blog/students-guide-to-raft/)

lab2是实现raft库，可以分为`选主`，`日志复制`，`日志持久化`，`快照`4个部分。
这部分的代码正确实现的关键点，在于按照论文中 figure 2 所阐述的规则严格实现，对于不理解的的地方规则可以仔细阅读论文对应的章节。除此之外也会有些细节需要注意，我会记录在文档中，以⚠️标记。具体实现部分的逻辑阐述，我会主要记录在代码注释中。
## 2A Leader eletion
通过率✅ 	1000/1000

raft使用心跳机制去触发选举，只要节点在每个时间周期收到有效心跳，就会保持Fllower的角色，否则开始选举。`心跳`就是不带有日志的空 AppendEntried RPCs请求
最初所有节点都是Follower，由于随机超时时间设计，会有一个节点最先触发选举，选举成功后会`立即`开始发送心跳给所有其他节点。
⚠️ after be Follower and be Leader:投票变成nil
## 2B Log replication
通过率✅	92/100

通过前面的选主得到一个Leader，Leader在收到客户端发送的log后，会将其存储到本地，然后同步到所节点，一旦有超过一半的节点`确认收到`（Follower将有效的log存储在本地后回复)，这条日志就会被标记为提交状态，便允许将此log应用到状态机。实现时，appendEntiedArgs 会把初始化后的空切片(len=0)反序列化为nil, 所以我使用了一个字段表示nil。

1. **如何确认提交的日志的索引？**

` Follower, Candidate`通过lastApplied, commitIndex 确认
`Leader` 通过matchIndex中超过一半peer能够达到的阈值来确认

2. **Test (2B): RPC counts aren't too high ...   TestCount2B:**

这个报错可以控制降低选举 rpc 发送频率来解决。

3. **快重传的理解**

appendLog失败后，Follower返回的的followerTerm和followerInder是给Leader用于更新 nextIndex:
`如果Leader的logIndex超出Follower的log长度范围:`followerInder是最后一个log的index + 1，											followerTerm = -1
`如果Leader的logTerm不匹配:`followerInderr 是冲突term的第一个log 或者第一个log(index = 1)
Leader 处理 快重传时:
`如果followerTerm = -1`直接将此节点的nextIndex更新为followerInder
`如果followerTerm和followerInde在Leader log 中存在`直接将其设置为nextIndex
`若不存在`按term为单位跳跃

4. 加速日志提交

由于lab3对客户端请求的响应速度（QPS）有要求，而服务端只会在相应raft日志提交后才会响应，底层实现是使用时间计时器定时同步日志和提交日志，所以底层raft库需要加快日志同步速度，进而继续加速日志同步速度。
这里我采用的方法是一旦收到新的日志就同步，一旦同步成功就提交。尽可能减少客户端请求的等待时间。

5. 出现 apply error 的原因

是某个节点先 apply 了日志 A，而另一个节点在相同的 index 提交了不同的日志 B，也就是说两个 server 同一个 index 上的 log 不一致了。
看到别人的博客说在发送 RPC 请求和处理 RPC 返回体时都需要判断一次自身状态。然后我在发送 RPC 之前先判断一下当前节点是否是 leader。

## 2C persistence
通过率✅	93/100

这部分的实现，只需要在涉及到持久化变量改变的地方调用持久化方法
## 2D log compaction
快照部分事实上并不单独属于raft的内容，它是raft与应用程序交互后获得的功能，因为raft本身并不理解快照中的内容，它需要上层应用程序的支持。暂时忽略这部分，把后面实验完成后，再把它和后续相关的快照系列一同完成。
# lab3 kvraft
此实验在log的基础上构建一个k v状态机，即键值存储。底层使用的go自带的map作为存储容器。
## 3A Key/value service without snapshots
通过率✅   95/100

[一致性读优化](https://cn.pingcap.com/blog/linearizability-and-raft)

[细节参考](https://github.com/OneSizeFitsQuorum/MIT6.824-2021/blob/master/docs/lab3.md)

⚠️每个请求日志请求都会有初始化一个channle来获取同步的结果，我们通过term 和 index去标识，如果单独使用logindex去标识，可能会导致这个标识不唯一。
leader一上线就可以提交一条空日志，以迅速恢复状态机的状态，因为leader重启上线后第一条log是读请求，此时leader状态机是没有状态的。并且，此时的状态机就可以不用通过Readlog的方式去Get key


