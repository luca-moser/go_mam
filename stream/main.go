package main

import (
	"crypto/rand"
	"fmt"
	"github.com/iotaledger/iota.go/api"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/luca-moser/mam"
	"time"
)

var ntruSK *mam.NTRUSK
var psk *mam.PSK
var iotaAPI *api.API

func main() {
	var err error
	iotaAPI, err = api.ComposeAPI(api.HTTPClientSettings{URI: "https://trinity.iota-tangle.io:14265"})
	must(err)
	spawnReader(spawnWriter())
}

func spawnReader(channelID trinary.Trytes) {
	// init MAM instance
	mamCtx := &mam.MAM{}
	must(mamCtx.Init(randomSeed()))

	stream := NewMAMReadStream(iotaAPI, mamCtx)
	must(stream.Open(channelID, []mam.PSK{*psk}, []mam.NTRUSK{*ntruSK}))
	for {
		select {
		case data := <-stream.Data:
			fmt.Println(string(data))
		case _ = <-stream.Errors:
			//fmt.Println("error from within stream:", err.Error())
		}
	}
}

func spawnWriter() trinary.Trytes {

	// init MAM instance
	mamCtx := &mam.MAM{}
	must(mamCtx.Init(randomSeed()))

	// generate keys
	ntruSK = mam.NewNTRUSK(mamCtx, "NTRUNTRUNTRU")
	psk = mam.NewPSK(mamCtx, "PSKPSKPSKPSKPSKPSK", "PSKPSKPSK")

	// init a new write stream
	stream := NewMAMWriteStream(iotaAPI, mamCtx)
	channelID, err := stream.Open()
	must(err)

	fmt.Printf("opening stream on: %s\n", channelID)

	// an unencrypted message which is readable by everyone with the right channel id
	go func() {
		for i := 1; ; i++ {
			var tailTx trinary.Trytes

			// an encrypted message to a group of recipients which share the same pre shared key
			groupMsg, err := NewMessage().Encrypted().Signed().
				Groups(*psk).Create([]byte(fmt.Sprintf("encrypted group message #%d", i)))
			must(err)

			tailTx, err = stream.Write(groupMsg)
			must(err)

			fmt.Printf("wrote encrypted group message with tail %s\n", tailTx)

			// an encrypted message to recipients through their NTRU public key
			recipientMsg, err := NewMessage().Encrypted().Signed().
				Recipients(ntruSK.PK).Create([]byte(fmt.Sprintf("encrypted single recipient message #%d", i)))
			must(err)

			tailTx, err = stream.Write(recipientMsg)
			must(err)

			fmt.Printf("wrote encrypted single recipient message with tail %s\n", tailTx)

			publicMsg, err := NewMessage().Create([]byte(fmt.Sprintf("public message #%d", i)))
			must(err)
			tailTx, err = stream.Write(publicMsg)
			must(err)
			fmt.Printf("wrote public message with tail %s\n", tailTx)
			<-time.After(time.Duration(10)*time.Second)
		}
	}()

	return channelID
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func randomSeed() trinary.Trytes {
	randBytes := make([]byte, 81)
	_, err := rand.Read(randBytes)
	must(err)
	var randSeed string
	alpabetLen := len(consts.TryteAlphabet)
	for _, randByte := range randBytes {
		randSeed += string(consts.TryteAlphabet[int(randByte)%alpabetLen])
	}
	return randSeed
}
