package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	ggio "github.com/gogo/protobuf/io"
	cid "github.com/ipfs/go-cid"
	libp2p "github.com/libp2p/go-libp2p"
	host "github.com/libp2p/go-libp2p-host"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"
	dhtpb "github.com/libp2p/go-libp2p-kad-dht/pb"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	ma "github.com/multiformats/go-multiaddr"
	cli "github.com/urfave/cli"
)

func setup(ctx context.Context, arg string) error {
	bef := time.Now()
	h, err := libp2p.New(ctx)
	if err != nil {
		return err
	}

	took := time.Since(bef)
	fmt.Printf("host construction took: %s\n", took)
	fmt.Printf("my peer ID is %s\n", h.ID().Pretty())

	parts := strings.Split(arg, "/ipfs/")
	pid, err = peer.IDB58Decode(parts[1])
	if err != nil {
		return err
	}

	addr, err := ma.NewMultiaddr(parts[0])
	if err != nil {
		return err
	}

	pi := pstore.PeerInfo{
		ID:    pid,
		Addrs: []ma.Multiaddr{addr},
	}

	bef = time.Now()
	if err := h.Connect(ctx, pi); err != nil {
		return err
	}
	took = time.Since(bef)
	fmt.Printf("connect took: %s\n", took)
	mhost = h
	return nil
}

var mhost host.Host
var pid peer.ID

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name: "key",
		},
		cli.StringFlag{
			Name:  "rpc",
			Value: "FIND_NODE",
		},
		cli.IntFlag{
			Name:  "count",
			Value: 5,
		},
	}
	app.Action = func(c *cli.Context) error {
		if err := setup(ctx, c.Args().First()); err != nil {
			return err
		}

		bef := time.Now()
		s, err := mhost.NewStream(ctx, pid, dhtopts.ProtocolDHT)
		if err != nil {
			return err
		}

		took := time.Since(bef)
		fmt.Printf("new stream took: %s\n", took)

		r := ggio.NewDelimitedReader(s, 2000000)
		w := ggio.NewDelimitedWriter(s)

		key := string(pid)
		if v := c.String("key"); v != "" {
			c, err := cid.Decode(v)
			if err != nil {
				return err
			}
			key = string(c.Bytes())
		}

		typ, ok := dhtpb.Message_MessageType_value[c.String("rpc")]
		if !ok {
			return fmt.Errorf("no such rpc type: %s", c.String("rpc"))
		}

		n := c.Int("count")
		/*
			// sequential requests
			timeRequest := func() {
				bef := time.Now()
				pmes := dhtpb.NewMessage(dhtpb.Message_MessageType(typ), key, 0)
				if err := w.WriteMsg(pmes); err != nil {
					panic(err)
				}

				took = time.Since(bef)
				fmt.Printf("write message took: %s\n", took)

				var resp dhtpb.Message
				if err := r.ReadMsg(&resp); err != nil {
					panic(err)
				}

				took = time.Since(bef)
				fmt.Printf("request took: %s\n", took)
				fmt.Println(resp)
			}

			for i := 0; i < n; i++ {
				timeRequest()
			}
		*/

		var sends []time.Time
		done := make(chan struct{})

		go func() {
			for i := 0; i < n; i++ {
				var resp dhtpb.Message
				if err := r.ReadMsg(&resp); err != nil {
					panic(err)
				}

				fmt.Printf("request took: %s\n", time.Since(sends[i]))
			}
			close(done)
		}()

		bef = time.Now()
		for i := 0; i < n; i++ {
			sends = append(sends, time.Now())
			pmes := dhtpb.NewMessage(dhtpb.Message_MessageType(typ), key, 0)
			if err := w.WriteMsg(pmes); err != nil {
				panic(err)
			}
		}
		<-done
		took = time.Since(bef)
		fmt.Printf("total time: %s\n", took)
		fmt.Printf("average time: %s\n", took/time.Duration(n))
		return nil
	}

	app.RunAndExitOnError()
}
