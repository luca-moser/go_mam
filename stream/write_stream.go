package main

import (
	"github.com/iotaledger/iota.go/api"
	"github.com/iotaledger/iota.go/bundle"
	"github.com/iotaledger/iota.go/converter"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/luca-moser/mam"
	"sync"
)

func NewMAMWriteStream(iotaAPI *api.API, m *mam.MAM) *MAMWriteStream {
	return &MAMWriteStream{iotaAPI: iotaAPI, m: m}
}

type MAMWriteStream struct {
	sync.Mutex
	iotaAPI          *api.API
	m                *mam.MAM
	currentChannelID trinary.Trytes
}

// Open opens up the MAM stream and returns the channel/address id on which the initial messages are written to.
func (stream *MAMWriteStream) Open() (trinary.Trytes, error) {
	channelID, err := stream.m.ChannelCreate(5)
	if err != nil {
		return "", err
	}
	stream.currentChannelID = channelID
	return channelID, nil
}

// Write writes the given message into the stream.
func (stream *MAMWriteStream) Write(msg *Message) (trinary.Trytes, error) {
	stream.Lock()
	defer stream.Unlock()
	bndl := bundle.Bundle{}

	var err error
	var msgID trinary.Trits
	bndl, msgID, err = stream.m.BundleWriteHeaderOnChannel(bndl, stream.currentChannelID, msg.psks, msg.ntruPks)
	if err != nil {
		return "", err
	}

	trytesData, err := converter.ASCIIToTrytes(string(msg.data))
	if err != nil {
		return "", err
	}

	var checksum mam.MsgChecksum
	if msg.integrity {
		checksum = mam.MsgChecksumMAC
	} else if msg.signed {
		checksum = mam.MsgChecksumSig
	} else {
		checksum = mam.MsgChecksumNone
	}

	bndl, err = stream.m.BundleWritePacket(msgID, trytesData, checksum, false, bndl)
	if err != nil {
		return "", err
	}

	bndl, err = broadcastMessage(stream.iotaAPI, bndl)
	if err != nil {
		return "", err
	}

	return bndl[0].Hash, nil
}

func broadcastMessage(iotaAPI *api.API, bndl bundle.Bundle) (bundle.Bundle, error) {
	tips, err := iotaAPI.GetTransactionsToApprove(3)
	if err != nil {
		return nil, err
	}

	bndl, err = bundle.Finalize(bndl)
	if err != nil {
		return nil, err
	}

	bndlTrytes := transaction.MustTransactionsToTrytes(bndl)
	for i, j := 0, len(bndlTrytes)-1; i < j; i, j = i+1, j-1 {
		bndlTrytes[i], bndlTrytes[j] = bndlTrytes[j], bndlTrytes[i]
	}
	readyBndlTrytes, err := iotaAPI.AttachToTangle(tips.TrunkTransaction, tips.BranchTransaction, 14, bndlTrytes)
	if err != nil {
		return nil, err
	}

	readyBndl, err := transaction.AsTransactionObjects(readyBndlTrytes, nil)
	if err != nil {
		return nil, err
	}

	_, err = iotaAPI.BroadcastTransactions(readyBndlTrytes...)
	if err != nil {
		return nil, err
	}

	return readyBndl, nil
}

func (stream *MAMWriteStream) Close() error {
	return stream.m.Destroy()
}