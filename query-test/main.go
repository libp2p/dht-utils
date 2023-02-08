package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	proto "github.com/gogo/protobuf/proto"
	dstore "github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"
	ipns "github.com/ipfs/go-ipns"
	writer "github.com/ipfs/go-log/writer"
	"github.com/ipfs/go-path"
	"github.com/libp2p/go-libp2p"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	peer "github.com/libp2p/go-libp2p-core/peer"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	record "github.com/libp2p/go-libp2p-record"
	ma "github.com/multiformats/go-multiaddr"
)

var bsaddrs = []string{
	"/ip4/104.131.131.82/tcp/4001/ipfs/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
	"/ip4/104.236.179.241/tcp/4001/ipfs/QmSoLPppuBtQSGwKDZT2M73ULpjvfd3aZ6ha4oFGL1KrGM",
	"/ip4/104.236.76.40/tcp/4001/ipfs/QmSoLV4Bbm51jM9C4gDYZQ9Cy3U6aXMJDAbzgu2fzaDs64",
	"/ip4/128.199.219.111/tcp/4001/ipfs/QmSoLSafTMBsPKadTEgaXctDQVcqN88CNLHXMkTNwMKPnu",
	"/ip4/178.62.158.247/tcp/4001/ipfs/QmSoLer265NRgSp2LA3dPaeykiS1J6DifTC88f5uVQKNAd",
}

func getRecordFromKey(sk ic.PrivKey) (peer.ID, []byte) {
	pid, err := peer.IDFromPublicKey(sk.GetPublic())
	if err != nil {
		panic(err)
	}

	pathval, err := path.ParsePath("QmXLF12bJ6XEsiqc4ZtBEMXCYVqwgHqbp4C929bGbJhuc4")
	if err != nil {
		panic(err)
	}

	e, err := ipns.Create(sk, []byte(pathval), 1, time.Now().Add(time.Hour*70), time.Hour)
	if err != nil {
		panic(err)
	}

	pukb, _ := sk.GetPublic().Bytes()
	e.PubKey = pukb

	data, err := proto.Marshal(e)
	if err != nil {
		panic(err)
	}
	return pid, data
}

func setupNode(ctx context.Context) (host.Host, *dht.IpfsDHT) {
	h, err := libp2p.New(ctx, libp2p.Defaults)
	if err != nil {
		panic(err)
	}

	ds := dsync.MutexWrap(dstore.NewMapDatastore())
	rt := dht.NewDHT(ctx, h, ds)

	validator := record.NamespacedValidator{
		"pk":   record.PublicKeyValidator{},
		"ipns": ipns.Validator{KeyBook: h.Peerstore()},
	}
	rt.Validator = validator

	var wg sync.WaitGroup
	for _, bsa := range bsaddrs {
		wg.Add(1)
		go func(bsa string) {
			defer wg.Done()
			a, err := ma.NewMultiaddr(bsa)
			if err != nil {
				panic(err)
			}

			pinfo, err := peer.AddrInfoFromP2pAddr(a)
			if err != nil {
				panic(err)
			}

			bef := time.Now()
			if err := h.Connect(ctx, *pinfo); err != nil {
				fmt.Println("connect to bootstrapper failed: ", err)
			}
			fmt.Printf("Connect(%s) took %s\n", pinfo.ID.Pretty(), time.Since(bef))
		}(bsa)
	}
	wg.Wait()
	return h, rt
}

func main() {
	el, err := NewEventsLogger("events")
	if err != nil {
		panic(err)
	}

	r, w := io.Pipe()

	go el.handleEvents(r)
	writer.WriterGroup.AddWriter(w)

	ctx := context.Background()

	h, rt := setupNode(ctx)

	fmt.Println("peerID is: ", h.ID().Pretty())

	sk, _, _ := ic.GenerateKeyPair(ic.RSA, 2048)
	pid, recval := getRecordFromKey(sk)
	k := "/ipns/" + string(pid)

	var snaps [][]*PeerDialLog
	var times []time.Duration

	for i := 0; i < 5; i++ {
		log.Println("starting ipnsput")
		bef := time.Now()
		if err := rt.PutValue(ctx, k, recval); err != nil {
			log.Println("ipns put failed: ", err)
		}
		took := time.Since(bef)
		fmt.Println("took: ", time.Since(bef))
		times = append(times, took)

		dials := el.Dials
		fmt.Println("total dials: ", len(dials))
		snaps = append(snaps, dials)
		el.Dials = nil
		el.DialsByParent = make(map[uint64][]DialAttempt)
	}

	for i, s := range snaps {
		fmt.Println(times[i], len(s))
	}

	fmt.Println("total connected peers: ", len(h.Network().Conns()))

	valout, err := rt.GetValue(ctx, k)
	if err != nil {
		fmt.Println("getvalue after put failed: ", err)
	}
	fmt.Println(valout)
}
