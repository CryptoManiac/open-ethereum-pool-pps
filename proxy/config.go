package proxy

import (
	"github.com/CryptoManiac/open-ethereum-pool/api"
	"github.com/CryptoManiac/open-ethereum-pool/payouts"
	"github.com/CryptoManiac/open-ethereum-pool/shifts"
	"github.com/CryptoManiac/open-ethereum-pool/policy"
	"github.com/CryptoManiac/open-ethereum-pool/storage"
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
	BlockRefreshInterval string `json:"blockRefreshInterval"`
	Difficulty           int64  `json:"difficulty"`
	MiningFee            float64 `json:"miningFee"`
	PoT_A                float64 `json:"potA"`
	PoT_Cap              float64 `json:"potCap"`
	StateUpdateInterval  string `json:"stateUpdateInterval"`
	CleanInterval        string `json:"cleanInterval"`
	HashrateExpiration   string `json:"hashrateExpiration"`

	Policy policy.Config `json:"policy"`

	MaxFails    int64 `json:"maxFails"`
	HealthCheck bool  `json:"healthCheck"`

	Stratum StratumEndpoint `json:"stratum"`
}

type StratumEndpoint struct {
	Enabled bool   `json:"enabled"`
	Listen  string `json:"listen"`
	Protocol string `json:"protocol"`
	NonceSpace []uint8 `json:"xnSpace"`
	MinDiffFloat float64 `json:"minDiffFloat"`
	NonceSize int `json:"xnSize"`
	Timeout string `json:"timeout"`
	MaxConn int    `json:"maxConn"`
}

type Upstream struct {
	Name    string `json:"name"`
	Url     string `json:"url"`
	Timeout string `json:"timeout"`
}
