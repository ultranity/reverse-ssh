package server

import (
	"fmt"
	"log"
	"net"
	"reverse-ssh/core"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

type ConnInfo struct {
	info core.ExtraInfo
	conn *gossh.ServerConn
}

var ConnList = make(map[string]ConnInfo)

func CreateExtraInfoHandler() ssh.ChannelHandler {
	return func(srv *ssh.Server, conn *gossh.ServerConn, newChan gossh.NewChannel, ctx ssh.Context) {
		var extraInfo core.ExtraInfo
		err := gossh.Unmarshal(newChan.ExtraData(), &extraInfo)
		newChan.Reject(gossh.Prohibited, "th4nkz")
		if err != nil {
			log.Printf("Could not parse extra info from %s", conn.RemoteAddr())
			return
		}
		log.Printf(
			"New connection from %s: %s on %s reachable via %s",
			conn.RemoteAddr(),
			extraInfo.CurrentUser,
			extraInfo.Hostname,
			extraInfo.ListeningAddress,
		)
		ConnList[extraInfo.ListeningAddress] = ConnInfo{extraInfo, conn}
		go func() {
			err := conn.Wait()
			log.Printf("conn from %s: %s on %s Closed %+v",
				conn.RemoteAddr(),
				extraInfo.CurrentUser,
				extraInfo.Hostname, err)
			delete(ConnList, extraInfo.ListeningAddress)
		}()
	}
}

func RunL(p *core.Params, server *ssh.Server) {
	log.Printf("Starting ssh server on :%d", p.LPORT)
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", p.LPORT))
	if err == nil {
		log.Printf("Success: listening on %s", ln.Addr().String())
	} else {
		log.Fatal(err)
	}
	defer ln.Close()
	log.Fatal(server.Serve(ln))
}
