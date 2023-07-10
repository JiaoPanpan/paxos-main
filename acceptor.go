package paxos

import (
	"log"
	"math/rand"
	"net"
	"net/rpc"
	"sync"
)

type Acceptor struct {
	mu            sync.Mutex
	localAddr     string      // 本地tcp地址
	learnerPeers  []string    // learner的tcp地址
	promiseID     float32     // 收到的最高proposeID
	acceptedID    float32     // 接受的proposeID
	acceptedValue interface{} // 接受的值
	listener      net.Listener
	isunreliable  bool // 用于模拟不可靠网络
}

func (a *Acceptor) getLearnerPeers() []string {
	a.mu.Lock()
	peers := a.learnerPeers
	a.mu.Unlock()
	return peers
}

func (a *Acceptor) getAddr() string {
	a.mu.Lock()
	addr := a.localAddr
	a.mu.Unlock()
	return addr
}

func (a *Acceptor) RecievePrepare(arg *PrepareMsg, reply *PromiseMsg) error {
	// logPrint("[acceptor %s RecievePrepare:%v ]", a.localAddr, arg)
	logPrint("[acceptor %s RecievePrepare:%v ]", a.localAddr, arg.ProposeID)

	reply.ProposeID = arg.ProposeID
	reply.AcceptorAddr = a.getAddr()
	//关于回应promise：如果当前accepter没接受过任何值，那应该返回promise: 相应id, 对应的值
	//如果接受过，则应该返回promise: 相应id，之前的最高id和其相应的值
	if arg.ProposeID > a.promiseID {
		a.promiseID = arg.ProposeID
		reply.Success = true
		var flag =false
		
		if a.acceptedID > 0 && a.acceptedValue != nil {
			flag =true
			logPrint("[acceptor %s SendPromise:%v,(%v,%v) ]", a.localAddr, a.promiseID,a.acceptedID,a.acceptedValue)//add
			reply.AccepedID = a.acceptedID
			reply.AccepedValue = a.acceptedValue
		}
		if !flag {
			logPrint("[acceptor %s SendPromise:%v ]", a.localAddr, a.promiseID)//add
		}
			
	}

	// PASS 持久化promise的数据
	return nil
}

func (a *Acceptor) RecieveAccept(arg *AcceptMsg, reply *AcceptedMsg) error {
	// logPrint("[acceptor %s RecieveAccept:%v ]", a.localAddr, arg)
	logPrint("[acceptor %s RecieveAccept:%v ]", a.localAddr, arg.ProposeID)

	reply.ProposeID = arg.ProposeID
	if arg.ProposeID == a.promiseID {
		//如果发过来值的这个proposer与a回应的一致：accepted应为id和值
		// logPrint("[acceptor %s SendAccept:%v ]", a.localAddr, arg)//add
		logPrint("[acceptor %s SendAccept:%v ]", a.localAddr, arg.ProposeID)//add

		reply.Success = true
		reply.AcceptorAddr = a.getAddr()
		a.promiseID = arg.ProposeID
		a.acceptedID = arg.ProposeID
		a.acceptedValue = arg.Value
		for _, learnerPeer := range a.getLearnerPeers() {
			callRpc(learnerPeer, "Learner", "RecieveAccepted", reply, &EmptyMsg{})
		}
	}
	// PASS 持久化accepted的数据
	return nil
}

func (a *Acceptor) startRpc() {
	rpcx := rpc.NewServer()
	rpcx.Register(a)
	l, e := net.Listen("tcp", a.localAddr)
	a.listener = l
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				continue
			}
			if a.isunreliable && rand.Int63()%1000 < 300 {
				conn.Close()
				continue
			}
			// i :=rand.Int63()%1000 < 300//debug
			// logPrint("断开连接%v",i)//debug
			// conn.Close()//debug
			// continue//debug

			go rpcx.ServeConn(conn)
		}
	}()
}

func (a *Acceptor) clean() {
	a.promiseID = 0
	a.acceptedID = 0
	a.acceptedValue = nil
}

func (a *Acceptor) close() {
	a.listener.Close()
}
