package proxy

import (
	"log"
	"math/big"
	"strconv"
	"strings"

	"github.com/CryptoManiac/ethash"
	"github.com/ethereum/go-ethereum/common"
)

var hasher = ethash.New()

func (s *ProxyServer) processShare(login, id, ip string, t *BlockTemplate, data []string) (bool, bool, []string) {
	nonce, _ := strconv.ParseUint(strings.Replace(data[0], "0x", "", -1), 16, 64)
	shareDiff := s.config.Proxy.Difficulty
	shareFee := s.config.Proxy.MiningFee
	potA := s.config.Proxy.PoT_A
	potCap := s.config.Proxy.PoT_Cap

	h, ok := t.headers[data[1]]
	if !ok {
		log.Printf("Stale share from %v@%v", login, ip)
		return false, false, data
	}

	share := Block{
		number:      h.height,
		hashNoNonce: common.HexToHash(data[1]),
		difficulty:  h.diff,
		nonce:       nonce,
		mixDigest:   common.HexToHash(data[2]),
	}

	// Verify validity against block and share target
	isShare, isBlock, actualDiff, mixHash := hasher.VerifyShare(share, big.NewInt(shareDiff))

	// Replace provided mixDigest with calculated one
	data[2] = mixHash.Hex()

	if !isShare {
		return false, false, data
	}

	if isBlock {
		ok, err := s.rpc().SubmitBlock(data)
		if err != nil {
			log.Printf("Block submission failure at height %v for %v: %v", h.height, t.Header, err)
		} else if !ok {
			log.Printf("Block rejected at height %v for %v", h.height, t.Header)
			return false, false, data
		} else {
			s.fetchBlockTemplate()
			exist, err := s.backend.WriteBlock(login, id, data, shareDiff, actualDiff, potA, potCap, shareFee, h.diff.Int64(), h.height, t.Height, s.hashrateExpiration)
			if exist {
				return true, false, data
			}
			if err != nil {
				log.Println("Failed to insert block candidate into backend:", err)
			} else {
				log.Printf("Inserted block %v to backend", h.height)
			}
			log.Printf("Block found by miner %v@%v at height %d", login, ip, h.height)
		}
	} else {
		exist, err := s.backend.WriteShare(login, id, data, shareDiff, actualDiff, potA, potCap, shareFee, h.diff.Int64(), h.height, t.Height, s.hashrateExpiration)
		if exist {
			return true, false, data
		}
		if err != nil {
			log.Println("Failed to insert share data into backend:", err)
		}
	}
	return false, true, data
}
