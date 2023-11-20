package bsc2topv2

import "github.com/ethereum/go-ethereum/common"

var BSCClientContract = common.HexToAddress("0xff00000000000000000000000000000000000003")

const (
	FATALTIMEOUT int64 = 24 //hours
	SUCCESSDELAY int64 = 10
	ERRDELAY     int64 = 10
	WAITDELAY    int64 = 60

	CONFIRM_NUM uint64 = 5
	BATCH_NUM   uint64 = 5
)
