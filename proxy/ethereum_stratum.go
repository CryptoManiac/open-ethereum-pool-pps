package proxy

import (
	"net"
	"log"
	"encoding/json"
	"bufio"
	"io"
	"fmt"
	"strconv"
	"time"
	"math"
	"math/rand"

	"github.com/thanhpk/randstr"
	"github.com/CryptoManiac/open-ethereum-pool/util"
	"strings"
)

// EthereumStratum job
type JobData struct {
	JobId uint64
	SeedHash string
	HeaderHash string
}

type JobQueue struct {
	topId uint64
	items []JobData
}

func (jq *JobQueue) Init() {
	if len(jq.items) == 0 {
		jq.items = []JobData {}
		for i := 0; i < MaxBacklog; i++ {
			jq.items = append(jq.items, JobData{})
		}
		jq.topId = 0
	}
}

func (jq *JobQueue) JobEnqueue(tmpl *BlockTemplate, job *JobData) {
	seedHash, headerHash := tmpl.Seed, tmpl.Header

	if (seedHash[0:2] == "0x") {
		seedHash = seedHash[2:]
	}
	if (headerHash[0:2] == "0x") {
		headerHash = headerHash[2:]
	}

	// Convert first 8 bytes of header hash to job ID
	jq.topId, _ = strconv.ParseUint(headerHash[:16], 16, 64)

	*job = JobData{
		JobId: jq.topId,
		SeedHash: seedHash,
		HeaderHash: headerHash,
	}

	copy(jq.items[0:], jq.items[1:])
	jq.items = jq.items[:MaxBacklog-1]
	jq.items = append(jq.items, *job)
}

func (jq *JobQueue) FindJob(JobId uint64, job *JobData) bool {
	for _, v := range jq.items {
		if v.JobId == JobId {
			*job = v
			return true
		}
	}
	return false
}

func (jq *JobQueue) GetTopJob(job *JobData) bool {
	if len(jq.items) > 0 {
		*job = jq.items[len(jq.items) - 1]
		return job.JobId == jq.topId
	}
	
	return false
}

func (s *ProxyServer) makePrefix() string {
	// Extranonce prefix
	var prefix int
	
	switch space := s.config.Proxy.Stratum.NonceSpace; len(space) {
	case 1:
		prefix = int(space[0])
	case 2:
		rand.Seed(time.Now().UnixNano())
		prefix = rand.Intn(int(space[1] - space[0])) + int(space[0])
	default:
		return ""
	}
	
	return fmt.Sprintf("%02x", prefix)
}

// Get unique extranonce
func (s *ProxyServer) GetExtraNonce() string {
	var extraNonce string

	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	for {
		prefix := s.makePrefix()
		extraNonce = prefix + randstr.Hex(s.nonceSize - len(prefix) / 2)
		found := false

		for m, _ := range s.sessions {
			if m.Extranonce == extraNonce {
				found = true
				break
			}
		}

		if !found {
			break
		}
	}
	
	return extraNonce
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

	for {
		conn, err := server.AcceptTCP()
		if err != nil {
			continue
		}
		conn.SetKeepAlive(true)

		ip, _, _ := net.SplitHostPort(conn.RemoteAddr().String())

		if s.policy.IsBanned(ip) || !s.policy.ApplyLimitPolicy(ip) {
			conn.Close()
			continue
		}
		n += 1
		cs := &Session{ conn: conn, ip: ip, Extranonce: s.GetExtraNonce(), Difficulty: s.config.Proxy.Difficulty }

		// Remember difficulty
		s.workMu.RLock()
		s.workDiff[cs.Extranonce] = &WorkDiff{Difficulty:cs.Difficulty, ToRemove: false}
		s.workMu.RUnlock()

		accept <- n
		go func(cs *Session) {
			err = s.handleESClient(cs)
			if err != nil {
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

func(cs *Session) getNotificationResponse(s *ProxyServer, id *json.RawMessage) EthStratumResp {
	result := make([]interface{}, 2)
	result[0] = []string { "mining.notify", randstr.Hex(16), "EthereumStratum/1.0.0" }
	result[1] = cs.Extranonce

	resp := EthStratumResp{
		Id:id,
		Result:result,
		Error: nil,
	}

	return resp
}

func(cs *Session) sendESError(id *json.RawMessage, message interface{}) error{
	cs.Mutex.Lock()
	defer cs.Mutex.Unlock()

	resp := EthStratumResp{Id: id, Error: message}
	return cs.enc.Encode(&resp)
}

func(cs *Session) sendESMessage(resp interface{})  error {
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
			cs.Extranonce + fmt.Sprintf("%x", job.JobId),
			job.SeedHash,
			job.HeaderHash,
			true,
		},
	}

	return cs.sendESMessage(resp)
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
		return cs.sendESMessage(resp)

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

		resp := EthStratumResp{Id:req.Id, Result:reply, Error:nil}
		if err := cs.sendESMessage(resp); err != nil{
			return err
		}

		floatDiff := float64(cs.Difficulty) / 4294967296.0
		if len(params) > 1 {
			userDiff, err2 := strconv.ParseFloat(params[1], 64)
			if err2 == nil && userDiff >= s.minDiffFloat && userDiff <= s.maxDiffFloat {
				userDiff = math.Floor(userDiff * 100) / 100
				cs.Difficulty = int64(math.Ceil(userDiff * 4294967296.0))
				s.workMu.RLock()
				s.workDiff[cs.Extranonce] = &WorkDiff{Difficulty: cs.Difficulty, ToRemove: false}
				s.workMu.RUnlock()
				floatDiff = math.Floor(userDiff * 100) / 100
			}
		}

		respReq := EthStratumReq{Method:"mining.set_difficulty", Params: []float64{ floatDiff }}
		if err := cs.sendESMessage(respReq); err != nil {
			return err
		}

		return cs.sendJob(s, req.Id)

	case "mining.extranonce.subscribe":
		resp := EthStratumResp{Id:req.Id, Result:true, Error:nil}
		if err := cs.sendESMessage(resp); err != nil{
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
		
		jobNonce := params[1][:s.nonceSize * 2]
		JobId, _ := strconv.ParseUint(params[1][s.nonceSize * 2:], 16, 64)

		s.workMu.RLock()
		jobDiff := cs.Difficulty
		rec, fine := s.workDiff[jobNonce]
		if fine {
			jobDiff = rec.Difficulty
		}
		s.workMu.RUnlock()

		var job JobData
		if !s.Jobs.FindJob(JobId, &job) {
			log.Printf("Unknown job %v from %v@%v", JobId, cs.login, cs.ip)
			return cs.sendESError(req.Id, "unknown job id")
		}

		params = []string{
			jobNonce + params[2],
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

			resp := cs.sendESMessage(EthStratumResp{
				Id: req.Id,
				Result: reply,
			})
			closeOnErr(resp)
		}
		go s.handleTCPSubmitRPC(cs, id, params, jobDiff, callback)
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
	s.Jobs.JobEnqueue(t, &job)
	s.jobsMu.RUnlock()

	count := len(s.sessions)
	log.Printf("Broadcasting new job %v to %v ethereum stratum miners", job.JobId, count)

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
					cs.Extranonce + fmt.Sprintf("%x", job.JobId),
					job.SeedHash,
					job.HeaderHash,
					true,
				},
			}

			err := cs.sendESMessage(resp)
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

