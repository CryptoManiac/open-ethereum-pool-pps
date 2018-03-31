package proxy

import (
	"github.com/sammy007/open-ethereum-pool/api"
	"github.com/sammy007/open-ethereum-pool/payouts"
	"github.com/sammy007/open-ethereum-pool/shifts"
	"github.com/sammy007/open-ethereum-pool/policy"
	"github.com/sammy007/open-ethereum-pool/storage"
)

type Config struct {
	Name                  string        `json:"name"`
	Proxy                 Proxy         `json:"proxy"`
	Api                   api.ApiConfig `json:"api"`
	Upstream              []Upstream    `json:"upstream"`
	UpstreamCheckInterval string        `json:"upstreamCheckInterval"`

	Threads int `json:"threads"`

	Coin  string         `json:"coin"`
	Redis storage.Config `json:"redis"`

	Payouts       payouts.PayoutsConfig  `json:"payouts"`
	Shifts        shifts.ShiftsConfig  `json:"shifts"`
}

type Proxy struct {
	Enabled              bool   `json:"enabled"`
	Listen               string `json:"listen"`
	LimitHeadersSize     int    `json:"limitHeadersSize"`
	LimitBodySize        int64  `json:"limitBodySize"`
	BehindReverseProxy   bool   `json:"behindReverseProxy"`
	BlockRefreshInterval string `json:"blockRefreshInterval"`
	Difficulty           int64  `json:"difficulty"`
	MiningFee            float64 `json:"miningFee"`
	PoT_A                float64 `json:"potA"`
	PoT_Cap              float64 `json:"potCap"`
	StateUpdateInterval  string `json:"stateUpdateInterval"`
	HashrateExpiration   string `json:"hashrateExpiration"`

	Policy policy.Config `json:"policy"`

	MaxFails    int64 `json:"maxFails"`
	HealthCheck bool  `json:"healthCheck"`

	Stratum Stratum `json:"stratum"`
}

type Stratum struct {
	Enabled bool   `json:"enabled"`
	Listen  string `json:"listen"`
	Timeout string `json:"timeout"`
	MaxConn int    `json:"maxConn"`
}

type Upstream struct {
	Name    string `json:"name"`
	Url     string `json:"url"`
	Timeout string `json:"timeout"`
}
