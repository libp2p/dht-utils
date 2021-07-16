package main

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	routing "github.com/libp2p/go-libp2p-core/routing"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-kad-dht"
	ma "github.com/multiformats/go-multiaddr"
)

var bsaddrs = []string{
	"/ip4/104.131.131.82/tcp/4001/ipfs/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
	"/ip4/104.236.179.241/tcp/4001/ipfs/QmSoLPppuBtQSGwKDZT2M73ULpjvfd3aZ6ha4oFGL1KrGM",
	"/ip4/128.199.219.111/tcp/4001/ipfs/QmSoLSafTMBsPKadTEgaXctDQVcqN88CNLHXMkTNwMKPnu",
	"/ip4/104.236.76.40/tcp/4001/ipfs/QmSoLV4Bbm51jM9C4gDYZQ9Cy3U6aXMJDAbzgu2fzaDs64",
	"/ip4/178.62.158.247/tcp/4001/ipfs/QmSoLer265NRgSp2LA3dPaeykiS1J6DifTC88f5uVQKNAd",
}

func computeQueryOverlapScore(peers [][]peer.ID) []int {
	counts := make(map[peer.ID]int)
	for _, ps := range peers {
		for _, p := range ps {
			counts[p]++
		}
	}

	out := make([]int, len(peers))
	for i, ps := range peers {
		for _, p := range ps {
			out[i] += counts[p]
		}
	}

	return out
}

func main() {
	var bspis []*peer.AddrInfo
	for _, a := range bsaddrs {
		maddr, err := ma.NewMultiaddr(a)
		if err != nil {
			panic(err)
		}
		ai, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			panic(err)
		}
		bspis = append(bspis, ai)
	}

	ctx := context.Background()

	ctx, events := routing.RegisterForQueryEvents(ctx)
	go func() {
		for e := range events {
			_ = e
			// if you want to see why things are broken
			fmt.Println("Event: ", e)
		}
	}()
	results := make([][]peer.ID, len(bspis))
	dials := make([][]peer.AddrInfo, len(bspis))
	for i := 0; i < len(bspis); i++ {
		fmt.Println("Running query bench round ", i)
		peers, addrs, err := RunSingleCrawl(ctx, "testkey", bspis[i:i+1])
		if err != nil {
			fmt.Println("failed to run query: ", err)
			continue
		}

		results[i] = peers
		dials[i] = addrs
	}

	scores := computeQueryOverlapScore(results)
	fmt.Println("Results:")
	for i, r := range results {
		fmt.Printf("%d: %d %d\n", i, len(r), scores[i])
	}
}

func RunSingleCrawl(ctx context.Context, k string, bootstrap []*peer.AddrInfo) ([]peer.ID, []peer.AddrInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	h, err := libp2p.New(ctx)
	if err != nil {
		return nil, nil, err
	}

	d, err := dht.New(ctx, h)
	if err != nil {
		return nil, nil, err
	}

	for _, bspi := range bootstrap {
		if err := h.Connect(ctx, *bspi); err != nil {
			return nil, nil, err
		}
	}

	// give the dht some breathing room to receive the identify events.
	time.Sleep(5 * time.Second)

	peers, err := d.GetClosestPeers(ctx, k)
	if err != nil {
		return nil, nil, err
	}

	var closest []peer.ID
	closest = append(closest, peers...)

	var successDials []peer.AddrInfo
	for _, c := range h.Network().Conns() {
		if c.Stat().Direction == network.DirOutbound {
			successDials = append(successDials, peer.AddrInfo{
				ID:    c.RemotePeer(),
				Addrs: []ma.Multiaddr{c.RemoteMultiaddr()},
			})
		}
	}

	return closest, successDials, nil
}
