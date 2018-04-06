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

func (s *ProxyServer) processShare(login, id, ip string, t *BlockTemplate, params []string) (bool, bool) {

	h, ok := t.headers[params[1]]
	if !ok {
		log.Printf("Stale share from %v@%v", login, ip)
		return false, false
	}

	hashNoNonce := common.HexToHash(params[1])
	mixDigest := common.HexToHash(params[2])
	nonce, _ := strconv.ParseUint(strings.Replace(params[0], "0x", "", -1), 16, 64)
	shareDiff := s.config.Proxy.Difficulty
	shareFee := s.config.Proxy.MiningFee
	potA := s.config.Proxy.PoT_A
	potCap := s.config.Proxy.PoT_Cap

	data := []string {
		params[0],
		params[1],
		params[2],
	}

	if hashNoNonce == mixDigest {
		mixDigest = hasher.ComputeMixDigest(t.Height, hashNoNonce, nonce)
		data[2] = mixDigest.Hex()
	}

	share := Block{
		number:      h.height,
		hashNoNonce: hashNoNonce,
		difficulty:  big.NewInt(shareDiff),
		nonce:       nonce,
		mixDigest:   mixDigest,
	}

	block := Block{
		number:      h.height,
		hashNoNonce: hashNoNonce,
		difficulty:  h.diff,
		nonce:       nonce,
		mixDigest:   mixDigest,
	}

	isShare, actualDiff := hasher.Verify(share)

	if !isShare {
		return false, false
	}

	isBlock, _ := hasher.Verify(block)

	if isBlock {
		ok, err := s.rpc().SubmitBlock(data)
		if err != nil {
			log.Printf("Block submission failure at height %v for %v: %v", h.height, t.Header, err)
		} else if !ok {
			log.Printf("Block rejected at height %v for %v", h.height, t.Header)
			return false, false
		} else {
			s.fetchBlockTemplate()
			exist, err := s.backend.WriteBlock(login, id, data, shareDiff, actualDiff, potA, potCap, shareFee, h.diff.Int64(), h.height, s.hashrateExpiration)
			if exist {
				return true, false
			}
			if err != nil {
				log.Println("Failed to insert block candidate into backend:", err)
			} else {
				log.Printf("Inserted block %v to backend", h.height)
			}
			log.Printf("Block found by miner %v@%v at height %d", login, ip, h.height)
		}
	} else {
		exist, err := s.backend.WriteShare(login, id, data, shareDiff, actualDiff, potA, potCap, shareFee, h.diff.Int64(), h.height, s.hashrateExpiration)
		if exist {
			return true, false
		}
		if err != nil {
			log.Println("Failed to insert share data into backend:", err)
		}
	}
	return false, true
}
