package proxy

import "encoding/json"

type JSONRpcReq struct {
	Id     *json.RawMessage `json:"id"`
	Method string           `json:"method"`
	Params *json.RawMessage `json:"params"`
}

type EthStratumReq struct  {
	Id     interface{} `json:"id"`
	Version string     `json:"jsonrpc"`
	Method string      `json:"method"`
	Params interface{} `json:"params"`
}

type EthStratumResp struct  {
	Id     interface{} `json:"id"`
	Version string     `json:"jsonrpc"`
	Result interface{} `json:"result"`
	Error interface{}  `json:"error,omitempty"`
}

type StratumReq struct {
	JSONRpcReq
	Worker string `json:"worker"`
}

// Stratum
type JSONPushMessage struct {
	// FIXME: Temporarily add ID for Claymore compliance
	Id      int64       `json:"id"`
	Version string      `json:"jsonrpc"`
	Result  interface{} `json:"result"`
}

type JSONRpcResp struct {
	Id      *json.RawMessage `json:"id"`
	Version string           `json:"jsonrpc"`
	Result  interface{}      `json:"result"`
	Error   interface{}      `json:"error,omitempty"`
}

type SubmitReply struct {
	Status string `json:"status"`
}

type ErrorReply struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
