package proxy

import (
	"net"
	"log"
	"encoding/json"
	"bufio"
	"io"
	"errors"
	"time"

	"github.com/thanhpk/randstr"
	"github.com/CryptoManiac/open-ethereum-pool/util"
	"strings"
)

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
		cs := &Session{conn: conn, ip: ip, Extranonce: randstr.Hex(2)}

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
	reply, errReply := s.handleGetWorkRPC(cs)
	if errReply != nil {
		return cs.sendESError(id, []string{
			string(errReply.Code),
			errReply.Message,
		})
	}

	for k, v := range reply {
		if v[0:2] == "0x" {
			reply[k] = v[2:]
		}
	}

	cs.JobDetails = jobDetails{
		JobID: randstr.Hex(4),
		SeedHash: reply[1],
		HeaderHash: reply[0],
	}

	resp := EthStratumReq{
		Method:"mining.notify",
		Params: []interface{}{
			cs.JobDetails.JobID,
			cs.JobDetails.SeedHash,
			cs.JobDetails.HeaderHash,
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

		if params[1] != "EthereumStratum/1.0.0"{
			log.Println("Unsupported stratum version from ", cs.ip)
			return cs.sendESError(req.Id, "unsupported ethereum version")
		}

		resp := cs.getNotificationResponse(s, req.Id)
		return cs.sendESResult(resp)

	case "mining.authorize":
		var params []string
		err := json.Unmarshal(*req.Params, &params)
		if err != nil {
			return errors.New("invalid params")
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
	case "mining.submit":
		var params []string
		if err := json.Unmarshal(*req.Params, &params); err != nil{
			return err
		}

		splitData := strings.Split(params[0], ".")
		id := "0"
		if len(splitData) > 1 {
			id = splitData[1]
		}

//		TODO: resolve jobid mismatching issue
//		if cs.JobDetails.JobID != params[1] {
//			return cs.sendESError(req.Id, "wrong job id")
//		}

		params = []string{
			cs.Extranonce + params[2],
			cs.JobDetails.HeaderHash,
			cs.JobDetails.HeaderHash,
		}
		
		for k, v := range params {
			if v[0:2] != "0x" {
				params[k] = "0x" + v
			}
		}

		reply, errReply := s.handleTCPSubmitRPC(cs, id, params)
		if errReply != nil {
			return cs.sendESError(req.Id, []string{
				string(errReply.Code),
				errReply.Message,
			})
		}
		resp := JSONRpcResp{
			Id: req.Id,
			Result: reply,
		}

		if err := cs.sendESResult(resp); err != nil{
			return err
		}

		return cs.sendJob(s, req.Id)

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

	count := len(s.sessions)
	log.Printf("Broadcasting new job to %v ethereum stratum miners", count)

	start := time.Now()
	bcast := make(chan int, 1024)
	n := 0

	for m, _ := range s.sessions {
		n++
		bcast <- n

		go func(cs *Session) {
			cs.JobDetails = jobDetails{
				JobID: randstr.Hex(4),
				SeedHash: t.Seed,
				HeaderHash: t.Header,
			}

			if (cs.JobDetails.SeedHash[0:2] == "0x") {
				cs.JobDetails.SeedHash = cs.JobDetails.SeedHash[2:]
			}
			if (cs.JobDetails.HeaderHash[0:2] == "0x") {
				cs.JobDetails.HeaderHash = cs.JobDetails.HeaderHash[2:]
			}

			resp := EthStratumReq{
				Method:"mining.notify",
				Params: []interface{}{
					cs.JobDetails.JobID,
					cs.JobDetails.SeedHash,
					cs.JobDetails.HeaderHash,
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

