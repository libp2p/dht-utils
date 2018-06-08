package main

import (
	"encoding/json"
	"io"
	"os"
	"time"

	tracer "github.com/ipfs/go-log/tracer"
)

type PeerDialLog struct {
	Dials      []DialAttempt `json:"dials"`
	Success    bool          `json:"success"`
	Duration   string        `json:"duration"`
	TargetPeer string        `json:"target"`
	Error      string        `json:"err"`
	Start      time.Time     `json:"start"`
}

type DialAttempt struct {
	TargetAddr string    `json:"targetAddr"`
	Result     string    `json:"result"`
	Error      string    `json:"error,omitempty"`
	Duration   string    `json:"duration"`
	Start      time.Time `json:"start"`
}

type EventsLogger struct {
	fi            *os.File
	enc           *json.Encoder
	DialsByParent map[uint64][]DialAttempt
	Dials         []*PeerDialLog
}

func NewEventsLogger(path string) (*EventsLogger, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return nil, err
	}

	enc := json.NewEncoder(f)

	return &EventsLogger{
		fi:            f,
		enc:           enc,
		DialsByParent: make(map[uint64][]DialAttempt),
	}, nil
}

func (el *EventsLogger) handleEvents(r io.Reader) {
	dec := json.NewDecoder(r)
	for {
		var iv interface{}
		if err := dec.Decode(&iv); err != nil {
			panic(err)
		}

		data, err := json.Marshal(iv)
		if err != nil {
			panic(err)
		}

		var ev tracer.LoggableSpan
		if err := json.Unmarshal(data, &ev); err != nil {
			panic(err)
		}

		switch ev.Operation {
		case "dialAddr":
			var errmsg string
			for _, l := range ev.Logs {
				if l.Field[0].Key == "error" {
					errmsg = l.Field[0].Value
				}
			}

			datt := DialAttempt{
				TargetAddr: ev.Tags["targetAddr"].(string),
				Duration:   ev.Duration.String(),
				Error:      errmsg,
				Start:      ev.Start,
			}

			el.DialsByParent[ev.ParentSpanID] = append(el.DialsByParent[ev.ParentSpanID], datt)

		case "swarmDialDo":
			rpeer, ok := ev.Tags["remotePeer"].(string)
			if !ok {
				panic("remotePeer field not set")
			}
			pdl := &PeerDialLog{
				TargetPeer: rpeer,
			}

			if ev.Tags["dial"] == "success" {
				pdl.Success = true
			}

			var errmsg string
			for _, l := range ev.Logs {
				if l.Field[0].Key == "error" {
					errmsg = l.Field[0].Value
				}
			}

			pdl.Error = errmsg
			pdl.Dials = el.DialsByParent[ev.SpanID]
			pdl.Duration = ev.Duration.String()
			pdl.Start = ev.Start

			el.Dials = append(el.Dials, pdl)
			if err := el.enc.Encode(pdl); err != nil {
				panic(err)
			}
		case "swarmDialAttemptStart":
			/*
				case "swarmDialAttemptSync":
					// this event tracks the entire life of a dial to a peer
					// It looks something like:
					// map[duration:1.396019259e+09 event:swarmDialAttemptSync peerID:QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ system:swarm2 time:2018-04-30T08:59:24.772706819Z
					// we should use this as a trigger to write out to the logfile this particular dial event
					fmt.Println(ev)
					p := ev.Tags["peerID"]
					pdl, ok := el.peers[p]
					if !ok {
						break
					}
					delete(el.peers, p)
					dur := ev.Duration
					pdl.Duration = dur.String()

					if err := el.enc.Encode(pdl); err != nil {
						panic(err)
					}
			*/
		}
	}
}
