package main

import (
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/iotaledger/iota.go/api"
	"github.com/iotaledger/iota.go/bundle"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/converter"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/luca-moser/mam"
	"math"
	"time"
)

func randSeed() trinary.Trytes {
	rnds := [81]byte{}
	_, err := rand.Read(rnds[:])
	must(err)
	var seed string
	for _, rnd := range rnds {
		seed += string(consts.TryteAlphabet[int(rnd)%(len(consts.TryteAlphabet) - 1)])
	}
	return seed
}

var seed = randSeed()

// write flags
var messagePayload = flag.String("msg", "this is a MAM message created through the Go bindings", "the payload message of the MAM message")
var jsonOutput = flag.Bool("json", false, "whether to output the generated txs in JSON to stdout")
var verbose = flag.Bool("v", false, "enable verbose mode")
var iterations = flag.Int("iterations", 1, "amount of write iterations to do")

// read flags
var trustedChannelID = flag.String("channel", "", "the id of the channel from which to fetch the messages")
var tailOfEndpointAnnouncementBundle = flag.String("tail-ep", "", "the tail tx hash of the bundle containing the endpoint announcement")
var tailTxOfBundle = flag.String("tail-msg", "", "the tail tx hash of the bundle containing the MAM message")

func main() {
	t := time.Now()
	flag.Parse()
	fmt.Printf("seed: %s\n", seed)
	switch len(*trustedChannelID) {
	case 0:
		fmt.Println("[write mode]")
		if *iterations == -1 {
			*iterations = math.MaxInt64
		}
		for i := 0; i < *iterations; i++ {
			write()
		}
	default:
		fmt.Println("[read mode]")
		read()
	}
	fmt.Printf("bye, took %v\n", time.Now().Sub(t))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func read() {
	iotaAPI, err := api.ComposeAPI(api.HTTPClientSettings{
		URI: "https://trinity.iota-tangle.io:14265",
	})
	must(err)
	mamAPI := &mam.MAM{}

	must(mamAPI.Init(seed))
	defer mamAPI.Destroy()

	// make channel trusted
	must(mamAPI.AddTrustedChannel(*trustedChannelID))

	// read bundle containing the endpoint announcement
	tailOfEndpointAnnouncementBundle, err := iotaAPI.GetBundle(*tailOfEndpointAnnouncementBundle)
	must(err)
	_, _, err = mamAPI.BundleRead(tailOfEndpointAnnouncementBundle)
	must(err)

	// now fetch actual bundle with the message
	bndl, err := iotaAPI.GetBundle(*tailTxOfBundle)
	must(err)

	payload, isLastPacket, err := mamAPI.BundleRead(bndl)
	must(err)

	payloadASCII, err := converter.TrytesToASCII(payload)
	must(err)
	fmt.Printf("payload: %s, is last packet: %v\n", payloadASCII, isLastPacket)
}

func write() {
	t := time.Now()
	iotaAPI, err := api.ComposeAPI(api.HTTPClientSettings{
		URI: "https://trinity.iota-tangle.io:14265",
	})
	must(err)
	mamAPI := &mam.MAM{}
	defer mamAPI.Destroy()

	must(mamAPI.Init(seed))

	start := time.Now()
	fmt.Println("creating channel...")
	channelID, err := mamAPI.ChannelCreate(5)
	must(err)
	fmt.Printf("created channel %s, took %s\n", channelID, time.Now().Sub(start))

	start = time.Now()
	fmt.Println("creating endpoint...")
	endpointID, err := mamAPI.EndpointCreate(5, channelID)
	must(err)
	fmt.Printf("created endpoint %s, took %s\n", endpointID, time.Now().Sub(start))

	// announcing endpoint
	fmt.Println("announcing endpoint...")
	bndl := bundle.Bundle{}
	bndl, msgID, err := mamAPI.BundleAnnounceEndpoint(bndl, channelID, endpointID, nil, nil)
	must(err)

	bndl = broadcastBundle(mamAPI, iotaAPI, bndl)
	fmt.Printf("MAM message announcing endpoint sent, took %v\nbundle hash: %s\ntail tx hash: %s\n", time.Now().Sub(t), bndl[0].Bundle, bndl[0].Hash)
	t = time.Now()

	// write actual message
	fmt.Println("creating actual MAM message containing payload...")
	fmt.Println("writing header...")
	bndl = bundle.Bundle{}
	bndl, msgID, err = mamAPI.BundleWriteHeaderOnEndpoint(bndl, channelID, endpointID, nil, nil)
	must(err)

	fmt.Println("writing packet...")
	textTrytes, err := converter.ASCIIToTrytes(*messagePayload)
	must(err)
	bndl, err = mamAPI.BundleWritePacket(msgID, textTrytes, mam.MsgChecksumSig, true, bndl)
	must(err)

	readyBndl := broadcastBundle(mamAPI, iotaAPI, bndl)
	tailTx := readyBndl[0]
	fmt.Printf("MAM message sent, took %v\nbundle hash: %s\ntail tx hash: %s\n", time.Now().Sub(t), tailTx.Bundle, tailTx.Hash)
}

func broadcastBundle(mamAPI *mam.MAM, iotaAPI *api.API, bndl bundle.Bundle) bundle.Bundle {
	fmt.Printf("bundle will contain %d txs\n", len(bndl))
	tips, err := iotaAPI.GetTransactionsToApprove(2)
	must(err)

	bndl, err = bundle.Finalize(bndl)
	must(err)

	bndlTrytes := transaction.MustTransactionsToTrytes(bndl)
	for i, j := 0, len(bndlTrytes)-1; i < j; i, j = i+1, j-1 {
		bndlTrytes[i], bndlTrytes[j] = bndlTrytes[j], bndlTrytes[i]
	}
	readyBndlTrytes, err := iotaAPI.AttachToTangle(tips.TrunkTransaction, tips.BranchTransaction, 14, bndlTrytes)
	must(err)

	readyBndl, err := transaction.AsTransactionObjects(readyBndlTrytes, nil)
	must(err)

	if *jsonOutput {
		readyBndlJSON, err := json.MarshalIndent(readyBndl, "", "   ")
		must(err)
		fmt.Print(string(readyBndlJSON))
	}

	_, err = iotaAPI.BroadcastTransactions(readyBndlTrytes...)
	must(err)

	return readyBndl
}
