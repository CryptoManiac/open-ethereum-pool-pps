package shifts

import (
	"log"
	"time"

	"github.com/sammy007/open-ethereum-pool/storage"
	"github.com/sammy007/open-ethereum-pool/util"
)

type ShiftsConfig struct {
	Enabled      bool   `json:"enabled"`
	Interval     string `json:"interval"`
}

type ShiftsProcessor struct {
	config   *ShiftsConfig
	backend  *storage.RedisClient
}

func NewShiftsProcessor(cfg *ShiftsConfig, backend *storage.RedisClient) *ShiftsProcessor {
	s := &ShiftsProcessor{config: cfg, backend: backend}
	return s
}

func (u *ShiftsProcessor) Start() {
	log.Println("Starting shifts")

	intv := util.MustParseDuration(u.config.Interval)
	timer := time.NewTimer(intv)
	log.Printf("Set shifts interval to %v", intv)

	// Immediately process payouts after start
	// u.process()
	timer.Reset(intv)

	go func() {
		for {
			select {
			case <-timer.C:
				u.process()
				timer.Reset(intv)
			}
		}
	}()
}

func (u *ShiftsProcessor) process() {
	users, _ := u.backend.GetPayees()
	shiftsDone := 0
	for _, login := range users {
		u.backend.WriteShift(login)
		shiftsDone++
	}
	log.Printf("%v shifts created", shiftsDone)
}
