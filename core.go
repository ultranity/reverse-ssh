// reverseSSH - a lightweight ssh server with a reverse connection feature
// Copyright (C) 2021  Ferdinor <ferdinor@mailbox.org>

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"reverse-ssh/client"
	"reverse-ssh/core"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/pkg/sftp"
)

func createLocalPortForwardingCallback(forbidden bool) ssh.LocalPortForwardingCallback {
	return func(ctx ssh.Context, dhost string, dport uint32) bool {
		if forbidden {
			log.Printf("Denying local port forwarding request %s:%d", dhost, dport)
			return false
		}
		log.Printf("Accepted forward to %s:%d", dhost, dport)
		return true
	}
}

func createReversePortForwardingCallback() ssh.ReversePortForwardingCallback {
	return func(ctx ssh.Context, host string, port uint32) bool {
		log.Printf("Attempt to bind at %s:%d granted", host, port)
		return true
	}
}

func createSessionRequestCallback(forbidden bool) ssh.SessionRequestCallback {
	return func(sess ssh.Session, requestType string) bool {
		if forbidden {
			log.Println("Denying shell/exec/subsystem request")
			return false
		}
		return true
	}
}

func createPasswordHandler(localPassword string) ssh.PasswordHandler {
	return func(ctx ssh.Context, pass string) bool {
		passed := pass == localPassword
		if passed {
			log.Printf("Successful authentication with password from %s@%s", ctx.User(), ctx.RemoteAddr().String())
		} else {
			log.Printf("Invalid password from %s@%s", ctx.User(), ctx.RemoteAddr().String())
		}
		return passed
	}
}

func createPublicKeyHandler(authorizedKey string) ssh.PublicKeyHandler {
	if authorizedKey == "" {
		return nil
	}

	return func(ctx ssh.Context, key ssh.PublicKey) bool {
		master, _, _, _, err := ssh.ParseAuthorizedKey([]byte(authorizedKey))
		if err != nil {
			log.Println("Encountered error while parsing public key:", err)
			return false
		}
		passed := bytes.Equal(key.Marshal(), master.Marshal())
		if passed {
			log.Printf("Successful authentication with ssh key from %s@%s", ctx.User(), ctx.RemoteAddr().String())
		} else {
			log.Printf("Invalid ssh key from %s@%s", ctx.User(), ctx.RemoteAddr().String())
		}
		return passed
	}
}

func createSFTPHandler() ssh.SubsystemHandler {
	return func(s ssh.Session) {
		server, err := sftp.NewServer(s)
		if err != nil {
			log.Printf("Sftp server init error: %s\n", err)
			return
		}

		log.Printf("New sftp connection from %s", s.RemoteAddr().String())
		if err := server.Serve(); err == io.EOF {
			server.Close()
			log.Println("Sftp connection closed by client")
		} else if err != nil {
			log.Println("Sftp server exited with error:", err)
		}
	}
}

var connecting = false

func RunRThenCheck(p *core.Params, server *ssh.Server, reversePWD string, reverseKey string, retry_max int) {
	target := net.JoinHostPort(p.LHOST, fmt.Sprintf("%d", p.LPORT))
	client.ActualPort = fmt.Sprintf("%d", p.BindPort)
	go runR(p.LUSER, target, p.BindPort, server, reversePWD, reverseKey, retryMax)
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		aim := net.JoinHostPort(p.LHOST, client.ActualPort)
		if !(connecting || checkConn(aim)) {
			connecting = true
			server.Close()
			go runR(p.LUSER, target, p.BindPort, server, reversePWD, reverseKey, retryMax)
			ticker.Stop()
		}
	}
}
func runR(LUSER, target string, BindPort uint, server *ssh.Server, reversePWD string, reverseKey string, retry_max int) {
	log.Printf("Dialling home via ssh to %s", target)
	ln, err := client.DialHomeAndListen(LUSER, target, BindPort, reversePWD, reverseKey, retryMax)
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()
	connecting = false
	log.Println(server.Serve(ln))
}

// check if the reverse port is open
func checkConn(target string) bool {
	var (
		err error
	)
	log.Printf("ping %s", target)
	_, err = net.DialTimeout("tcp", target, 5*time.Second)
	if err != nil {
		log.Println(err)
		return false
	}
	log.Println("alive")
	return true
}
