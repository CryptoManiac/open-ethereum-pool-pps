package proxy

import (
	"net"
	"log"
	"encoding/json"
	"bufio"
	"io"
	"fmt"
	"time"

	"github.com/thanhpk/randstr"
	"github.com/CryptoManiac/open-ethereum-pool/util"
	"strings"
)

// EthereumStratum job
type JobData struct {
	JobID string
	SeedHash string
	HeaderHash string
}

type JobQueue struct {
	topId int
	items []JobData
}


func (jq *JobQueue) Init() {
	if len(jq.items) == 0 {
		jq.items = []JobData {}
		for i := 0; i < MaxBacklog; i++ {
			jq.items = append(jq.items, JobData{})
		}
		jq.topId = 1
	}
}

func (jq *JobQueue) JobEnqueue(seedHash, headerHash string, job *JobData) {
	if (seedHash[0:2] == "0x") {
		seedHash = seedHash[2:]
	}
	if (headerHash[0:2] == "0x") {
		headerHash = headerHash[2:]
	}

	jq.topId += 1;

	*job = JobData{
		JobID: fmt.Sprintf("%x", jq.topId),
		SeedHash: seedHash,
		HeaderHash: headerHash,
	}

	copy(jq.items[0:], jq.items[1:])
	jq.items = jq.items[:MaxBacklog-1]
	jq.items = append(jq.items, *job)
}

func (jq *JobQueue) FindJob(JobID string, job *JobData) bool {
	for _, v := range jq.items {
		if v.JobID == JobID {
			*job = v
			return true
		}
	}
	return false
}

func (jq *JobQueue) GetTopJob(job *JobData) bool {
	if len(jq.items) > 0 {
		*job = jq.items[len(jq.items) - 1]
		return job.JobID == fmt.Sprintf("%x", jq.topId)
	}
	
	return false
}

type NoncePool struct {
	nonceSize int
	nonces []string
}

func (p *NoncePool) Init(nonceSize int, prefixSpace []uint8) {
	makeNonces := func(prefixes []string, nonceSize int) []string {
		ranges := []int {0xff, 0xffff, 0xffffff}
		if nonceSize < 1 || nonceSize > 3 {
			nonceSize = 2
		}
		var nonces []string

		if len(prefixes) > 0 {
			format := fmt.Sprintf("%%0%dx", (nonceSize - 1) * 2)
			for i := range prefixes {
				for j := 0; j < ranges[nonceSize - 2]; j++ {
					nonces = append(nonces, prefixes[i] + fmt.Sprintf(format, j))
				}
			}
		} else {
			format := fmt.Sprintf("%%0%dx", nonceSize * 2)
			for i := 0; i < ranges[nonceSize - 1]; i++ {
				nonces = append(nonces, fmt.Sprintf(format, i))
			}
		}
		
		return nonces
	}
	
	var prefixes []string

	switch len(prefixSpace) {
	case 1:
		prefixes = make([]string, 1)
		prefixes[0] = fmt.Sprintf("%02x", prefixSpace[0])
	case 2:
		prefixes = make([]string, int(prefixSpace[1] - prefixSpace[0]))
		for i := range prefixes {
			prefixes[i] = fmt.Sprintf("%02x", int(prefixSpace[0]) + i)
		}
	default:
		prefixes = make([]string, 0)
	}
	
	p.nonces = makeNonces(prefixes, nonceSize)
}

func (p *NoncePool) peekNonce() string {
	if len(p.nonces) == 0 {
		return ""
	}
	var nonce string
	nonce, p.nonces = p.nonces[0], p.nonces[1:]
	return nonce
}

func (p *NoncePool) keepNonce(nonce string) {
	p.nonces = append(p.nonces, nonce)
}

func (s *ProxyServer) ListenES(){
	timeout := util.MustParseDuration(s.config.Proxy.Stratum.Timeout)
	s.timeout = timeout

	addr, err := net.ResolveTCPAddr("tcp4", s.config.Proxy.Stratum.Listen)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	server, err := net.ListenTCP("tcp4", addr)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	defer server.Close()

	log.Printf("Ethereum Stratum listening on %s", s.config.Proxy.Stratum.Listen)
	var accept = make(chan int, s.config.Proxy.Stratum.MaxConn)
	n := 0

	// Init jobs queue
	s.Jobs = &JobQueue{}
	s.Jobs.Init()
	
	s.NoncePool = &NoncePool{}
	s.NoncePool.Init(s.config.Proxy.Stratum.NonceSize, s.config.Proxy.Stratum.NonceSpace)

	for {
		conn, err := server.AcceptTCP()
		if err != nil {
			continue
		}

		conn.SetKeepAlive(true)

		ip, _, _ := net.SplitHostPort(conn.RemoteAddr().String())

		s.noncesMu.RLock()
		nonce := s.NoncePool.peekNonce()
		s.noncesMu.RUnlock()
		if s.policy.IsBanned(ip) || !s.policy.ApplyLimitPolicy(ip) || nonce == "" {
			conn.Close()
			continue
		}
		n += 1
		cs := &Session{ conn: conn, ip: ip, Extranonce: nonce }

		accept <- n
		go func(cs *Session) {
			err = s.handleESClient(cs)
			if err != nil {
				s.noncesMu.RLock()
				s.NoncePool.keepNonce(cs.Extranonce)
				s.noncesMu.RUnlock()
				s.removeSession(cs)
				conn.Close()
			}
			<-accept
		}(cs)
	}
}

func (s *ProxyServer) handleESClient(cs *Session) error {
	cs.enc = json.NewEncoder(cs.conn)
	connbuff := bufio.NewReaderSize(cs.conn, MaxReqSize)
	s.setDeadline(cs.conn)

	for {
		data, isPrefix, err := connbuff.ReadLine()
		if isPrefix {
			log.Printf("Socket flood detected from %s", cs.ip)
			s.policy.BanClient(cs.ip)
			return err
		} else if err == io.EOF {
			log.Printf("Client %s disconnected", cs.ip)
			s.removeSession(cs)
			break
		} else if err != nil {
			log.Printf("Error reading from socket: %v", err)
			return err
		}

		if len(data) > 1 {
			var req StratumReq
			err = json.Unmarshal(data, &req)
			if err != nil {
				s.policy.ApplyMalformedPolicy(cs.ip)
				log.Printf("Malformed stratum request from %s: %v", cs.ip, err)
				return err
			}
			s.setDeadline(cs.conn)
			err = cs.handleESMessage(s, &req)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func(cs *Session) getNotificationResponse(s *ProxyServer, id *json.RawMessage) JSONRpcResp {
	result := make([]interface{}, 2)
	param1 := make([]string, 3)
	param1[0] = "mining.notify"
	param1[1] = randstr.Hex(16)
	param1[2] = "EthereumStratum/1.0.0"
	result[0] = param1
	result[1] = cs.Extranonce

	resp := JSONRpcResp{
		Id:id,
		Version:"EthereumStratum/1.0.0",
		Result:result,
		Error: nil,
	}

	return resp
}

func(cs *Session) sendESError(id *json.RawMessage, message interface{}) error{
	cs.Mutex.Lock()
	defer cs.Mutex.Unlock()

	resp := JSONRpcResp{Id: id, Error: message}
	return cs.enc.Encode(&resp)
}

func(cs *Session) sendESResult(resp JSONRpcResp)  error {
	cs.Mutex.Lock()
	defer cs.Mutex.Unlock()
	return cs.enc.Encode(&resp)
}

func(cs *Session) sendESReq(resp EthStratumReq)  error {
	cs.Mutex.Lock()
	defer cs.Mutex.Unlock()

	return cs.enc.Encode(&resp)
}

func(cs *Session) sendJob(s *ProxyServer, id *json.RawMessage) error {
	s.jobsMu.RLock()
	job := JobData {}
	if !s.Jobs.GetTopJob(&job) {
		return cs.sendESError(id, "unable to get current job")
	}
	s.jobsMu.RUnlock()

	resp := EthStratumReq{
		Method:"mining.notify",
		Params: []interface{}{
			job.JobID,
			job.SeedHash,
			job.HeaderHash,
			true,
		},
	}

	return cs.sendESReq(resp)
}

func (cs *Session) handleESMessage(s *ProxyServer, req *StratumReq) error {
	// Handle RPC methods
	switch req.Method {
	case "mining.subscribe":
		var params []string
		err := json.Unmarshal(*req.Params, &params)
		if err != nil {
			log.Println("Malformed stratum request params from", cs.ip)
			return err
		}
		if len(params) != 2 {
			log.Println("Malformed mining.subscribe request params from", cs.ip)
			return cs.sendESError(req.Id, "Invalid params")
		}
		if params[1] != "EthereumStratum/1.0.0"{
			log.Println("Unsupported stratum version from ", cs.ip)
			return cs.sendESError(req.Id, "unsupported ethereum version")
		}
		resp := cs.getNotificationResponse(s, req.Id)
		return cs.sendESResult(resp)

	case "mining.authorize":
		var params []string
		err := json.Unmarshal(*req.Params, &params)
		if err != nil || len(params) < 1 {
			log.Println("Malformed mining.authorize request params from", cs.ip)
			return cs.sendESError(req.Id, "Invalid params")
		}
		splitData := strings.Split(params[0], ".")
		params[0] = splitData[0]
		reply , errReply := s.handleLoginRPC(cs, params, req.Worker)
		if errReply != nil {
			return cs.sendESError(req.Id, []string{
				string(errReply.Code),
				errReply.Message,
			})
		}

		resp := JSONRpcResp{Id:req.Id, Result:reply, Error:nil}
		if err := cs.sendESResult(resp); err != nil{
			return err
		}

		paramsDiff := []float64{
			float64(s.config.Proxy.Difficulty) / 4294967296,
		}
		respReq := EthStratumReq{Method:"mining.set_difficulty", Params:paramsDiff}
		if err := cs.sendESReq(respReq); err != nil {
			return err
		}

		return cs.sendJob(s, req.Id)

	case "mining.extranonce.subscribe":
		resp := JSONRpcResp{Id:req.Id, Result:true, Error:nil}
		if err := cs.sendESResult(resp); err != nil{
			return err
		}

		return nil

	case "mining.submit":
		var params []string
		if err := json.Unmarshal(*req.Params, &params); err != nil{
			return err
		}
		
		if len(params) != 3 {
			log.Println("Malformed mining.submit request params from", cs.ip)
			return cs.sendESError(req.Id, "Invalid params")
		}
		
		splitData := strings.Split(params[0], ".")
		id := "0"
		if len(splitData) > 1 {
			id = splitData[1]
		}

		var job JobData
		if !s.Jobs.FindJob(params[1], &job) {
			log.Printf("Unknown job %v from %v@%v", params[1], cs.login, cs.ip)
			return cs.sendESError(req.Id, "unknown job id")
		}

		params = []string{
			cs.Extranonce + params[2],
			job.HeaderHash,
			"0000000000000000000000000000000000000000000000000000000000000000",
		}
		
		for k, v := range params {
			if v[0:2] != "0x" {
				params[k] = "0x" + v
			}
		}

		callback := func(reply bool, errReply *ErrorReply) {
			closeOnErr := func(err error) {
				if err != nil {
					// Close connection if there are any errors
					s.noncesMu.RLock()
					s.NoncePool.keepNonce(cs.Extranonce)
					s.noncesMu.RUnlock()
					cs.conn.Close()
					s.removeSession(cs)
				}
			}

			if errReply != nil {
				closeOnErr(cs.sendESError(req.Id, []string{
					string(errReply.Code),
					errReply.Message,
				}))
				return
			}

			resp := cs.sendESResult(JSONRpcResp{
				Id: req.Id,
				Result: reply,
			})
			closeOnErr(resp)
		}
		go s.handleTCPSubmitRPC(cs, id, params, callback)
		return nil

	default:
		errReply := s.handleUnknownRPC(cs, req.Method)
		return cs.sendESError(req.Id, []string{
			string(errReply.Code),
			errReply.Message,
		})
	}
}

func (s *ProxyServer) broadcastNewESJobs() {
	t := s.currentBlockTemplate()
	if t == nil || len(t.Header) == 0 || s.isSick() {
		return
	}

	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	s.jobsMu.RLock()
	job := JobData{}
	s.Jobs.JobEnqueue(t.Seed, t.Header, &job)
	s.jobsMu.RUnlock()

	count := len(s.sessions)
	log.Printf("Broadcasting new job %s to %v ethereum stratum miners", job.JobID, count)

	start := time.Now()
	bcast := make(chan int, 1024)
	n := 0

	for m, _ := range s.sessions {
		n++
		bcast <- n

		go func(cs *Session) {

			resp := EthStratumReq{
				Method:"mining.notify",
				Params: []interface{}{
					job.JobID,
					job.SeedHash,
					job.HeaderHash,
					true,
				},
			}

			err := cs.sendESReq(resp)
			<-bcast
			if err != nil {
				log.Printf("Job transmit error to %v@%v: %v", cs.login, cs.ip, err)
				s.removeSession(cs)
			} else {
				s.setDeadline(cs.conn)
			}
		}(m)
	}
	log.Printf("Jobs broadcast finished %s", time.Since(start))
}

