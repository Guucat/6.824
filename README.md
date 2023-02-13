# 6.824
MIT6.824分布式系统课程实验的个人实现代码，课程号现已更名为[6.5840](https://pdos.csail.mit.edu/6.824/index.html)<br />整个实验分为两部分，第一部分是lab1，实现`MapReduce并行计算系统`来统计词频，第二部分是lab2 - lab4，实现raft共识算法，并在此基础上实现一个`分布式KV存储系统`。
<a name="u12dB"></a>
# lab2 Raft
参考：[raft论文](https://pdos.csail.mit.edu/6.824/papers/raft-extended.pdf)， [guide](https://thesquareplanet.com/blog/students-guide-to-raft/)<br />lab2是实现raft库，可以分为`选主`，`日志复制`，`日志持久化`，`快照`4个部分。<br />这部分的代码正确实现的关键点，在于按照论文中 figure 2 所阐述的规则严格实现，对于不理解的的地方规则可以仔细阅读论文对应的章节。除此之外也会有些细节需要注意，我会记录在文档中，以⚠️标记。具体实现部分的逻辑阐述，我会主要记录在代码注释中。
<a name="hJPnj"></a>
## 2A Leader eletion
通过率✅ 	98/100<br />raft使用心跳机制去触发选举，只要节点在每个时间周期收到有效心跳，就会保持Fllower的角色，否则开始选举。`心跳`就是不带有日志的空 AppendEntried RPCs请求<br />最初所有节点都是Follower，由于随机超时时间设计，会有一个节点最先触发选举，选举成功后会`立即`开始发送心跳给所有其他节点。<br />⚠️ after be Follower and be Leader:投票变成nil
<a name="NWcwV"></a>
## 2B Log replication
通过率✅	92/100<br />      通过前面的选主得到一个Leader，Leader在收到客户端发送的log后，会将其存储到本地，然后同步到所节点，一旦有超过一半的节点`确认收到`（Follower将有效的log存储在本地后回复)，这条日志就会被标记为提交状态，便允许将此log应用到状态机。实现时，appendEntiedArgs 会把初始化后的空切片(len=0)反序列化为nil, 所以我使用了一个字段表示nil。

1. **如何确认提交的日志的索引？**

` Follower, Candidate`通过lastApplied, commitIndex 确认<br />`Leader` 通过matchIndex中超过一半peer能够达到的阈值来确认

2. **Test (2B): RPC counts aren't too high ...   TestCount2B:**

这个报错可以控制降低选举 rpc 发送频率来解决。

3. **快重传的理解**

appendLog失败后，Follower返回的的followerTerm和followerInder是给Leader用于更新 nextIndex:<br />`如果Leader的logIndex超出Follower的log长度范围:`followerInder是最后一个log的index + 1，											followerTerm = -1<br />`如果Leader的logTerm不匹配:`followerInderr 是冲突term的第一个log 或者第一个log(index = 1)<br />Leader 处理 快重传时:<br />`如果followerTerm = -1`直接将此节点的nextIndex更新为followerInder<br />`如果followerTerm和followerInde在Leader log 中存在`直接将其设置为nextIndex<br />`若不存在`按term为单位跳跃
<a name="qg4eV"></a>
### 2C persistence
通过率✅	36/100 怎么低了好多🥹<br />这部分的实现，只需要在涉及到持久化变量改变的地方调用持久化方法
<a name="R2xRo"></a>
### 2D log compaction
快照部分事实上并不单独属于raft的内容，它是raft与应用程序交互后获得的功能，因为raft本身并不理解快照中的内容，它需要上层应用程序的支持。暂时忽略这部分，把后面实验完成后，再把它和后续相关的快照系列一同完成。
