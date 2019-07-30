package main

import (
	"github.com/iotaledger/iota.go/api"
	"github.com/iotaledger/iota.go/checksum"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/converter"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/luca-moser/mam"
	"sync"
	"time"
)

func NewMAMReadStream(iotaAPI *api.API, m *mam.MAM) *MAMReadStream {
	return &MAMReadStream{
		iotaAPI: iotaAPI, m: m,
		Data:   make(chan []byte),
		Errors: make(chan error),
	}
}

type MAMReadStream struct {
	sync.Mutex
	iotaAPI          *api.API
	m                *mam.MAM
	currentChannelID trinary.Trytes
	Data             chan []byte
	Errors           chan error
}

func (stream *MAMReadStream) Open(channelID trinary.Trytes, preSharedKeys []mam.PSK, ntruSecretKeys []mam.NTRUSK) error {
	if err := stream.m.AddTrustedChannel(channelID); err != nil {
		return err
	}
	if preSharedKeys != nil {
		for x := range preSharedKeys {
			if err := stream.m.AddPreSharedKey(&preSharedKeys[x]); err != nil {
				return err
			}
		}
	}
	if ntruSecretKeys != nil {
		for x := range ntruSecretKeys {
			if err := stream.m.AddNTRUSecretKey(&ntruSecretKeys[x]); err != nil {
				return err
			}
		}
	}
	interval := time.Duration(5) * time.Second
	addrWithChecksum, err := checksum.AddChecksum(channelID, true, consts.AddressChecksumTrytesSize)
	if err != nil {
		return err
	}
	seen := map[string]struct{}{}
	go func() {
		for {
			<-time.After(interval)
			hashes, err := stream.iotaAPI.FindTransactions(api.FindTransactionsQuery{
				Addresses: trinary.Hashes{addrWithChecksum},
			})
			if err != nil {
				select {
				case stream.Errors <- err:
				default:
				}
			}

			for _, txHash := range hashes {
				if _, ok := seen[txHash]; ok {
					continue
				}
				bndl, err := stream.iotaAPI.GetBundle(txHash)
				if err != nil {
					select {
					case stream.Errors <- err:
					default:
					}
				}

				if bndl == nil {
					continue
				}

				payload, _, err := stream.m.BundleRead(bndl)
				if err != nil {
					seen[txHash] = struct{}{}
					select {
					case stream.Errors <- err:
					default:
					}
				}
				if len(payload) == 0{
					continue
				}

				for x := range bndl {
					seen[bndl[x].Hash] = struct{}{}
				}

				asciiTrytes, err := converter.TrytesToASCII(payload)
				if err != nil {
					select {
					case stream.Errors <- err:
					default:
					}
				}
				stream.Data <- []byte(asciiTrytes)
			}

		}
	}()

	return nil
}
