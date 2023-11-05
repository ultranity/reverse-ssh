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
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/pkg/sftp"
	gossh "golang.org/x/crypto/ssh"
)

type params struct {
	LUSER    string
	LHOST    string
	LPORT    uint
	BindPort uint
	Listen   bool
	shell    string
	noShell  bool
	verbose  bool
}

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

func dialHomeAndListen(username string, address string, homeBindPort uint, reversePWD string, reverseKey string, retry_max int) (net.Listener, error) {
	var (
		err    error
		client *gossh.Client
	)
	key, err := gossh.ParsePrivateKey([]byte(reverseKey))
	if err != nil {
		return nil, err
	}
	config := &gossh.ClientConfig{
		User: username,
		Auth: []gossh.AuthMethod{
			gossh.PublicKeys(key),
			gossh.Password(reversePWD),
		},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
	}

	// Attempt to connect with localPassword initially and keep asking for password on failure
	var fail_count = 0
	var timeout = 10 * time.Second
	for {
		client, err = gossh.Dial("tcp", address, config)
		if err == nil {
			break
		} else if retry_max < 0 || fail_count < retry_max {
			time.Sleep(timeout)
			log.Println(err)
			log.Printf("retry %d/%d", fail_count, retry_max)
			fail_count++
			if timeout < 6*time.Hour {
				timeout *= 2
			} else {
				timeout = 2 * time.Hour
			}
		} else {
			return nil, err
		}
	}
	ln, err := client.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", homeBindPort))
	if err != nil {
		return nil, err
	}
	actualPort = strings.Split(ln.Addr().String(), ":")[1]

	//TODO: lnu, err := client.Listen("unix", "/tmp/ssh-reverse.sock") //必须为绝对路径
	log.Printf("Success: up on %s", actualPort)

	// Attempt to send extra info back home in the info message of an extra ssh channel
	//TODO: sendExtraInfo(client, lnu.Addr().String())
	sendExtraInfo(client, ln.Addr().String())

	return ln, nil
}

type ExtraInfo struct {
	CurrentUser      string
	Hostname         string
	ListeningAddress string
}

func sendExtraInfo(client *gossh.Client, listeningAddress string) {

	extraInfo := ExtraInfo{ListeningAddress: listeningAddress}

	if usr, err := user.Current(); err != nil {
		extraInfo.CurrentUser = "ERROR"
	} else {
		extraInfo.CurrentUser = usr.Username
	}
	if hostname, err := os.Hostname(); err != nil {
		extraInfo.Hostname = "ERROR"
	} else {
		extraInfo.Hostname = hostname
	}

	newChan, newReq, err := client.OpenChannel("rs-info", gossh.Marshal(&extraInfo))
	//// The receiving end is expected to reject the channel, so "th4nkz" is a sign of success and we ignore it
	if err != nil && !strings.Contains(err.Error(), "th4nkz") {
		log.Printf("Could not create info channel: %+v", err)
	}

	// If the channel is actually accepted, just close it again
	if err == nil {
		go gossh.DiscardRequests(newReq)
		newChan.Close()
	}
}

func createExtraInfoHandler() ssh.ChannelHandler {
	return func(srv *ssh.Server, conn *gossh.ServerConn, newChan gossh.NewChannel, ctx ssh.Context) {
		var extraInfo ExtraInfo
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
		go func() {
			err := conn.Wait()
			log.Printf("conn from %s: %s on %s Closed %+v",
				conn.RemoteAddr(),
				extraInfo.CurrentUser,
				extraInfo.Hostname, err)
		}()
	}
}

func runL(p *params, server *ssh.Server) {
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

var actualPort string
var connecting = false

func runRAndCheck(p *params, server *ssh.Server) {
	target := net.JoinHostPort(p.LHOST, fmt.Sprintf("%d", p.LPORT))
	actualPort = fmt.Sprintf("%d", p.BindPort)
	go runR(p.LUSER, target, p.BindPort, server)
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		aim := net.JoinHostPort(p.LHOST, actualPort)
		if !(connecting || checkConn(aim)) {
			connecting = true
			server.Shutdown(context.TODO())
			go runR(p.LUSER, target, p.BindPort, server)
			ticker.Stop()
		}
	}
}
func runR(LUSER, target string, BindPort uint, server *ssh.Server) {
	log.Printf("Dialling home via ssh to %s", target)
	ln, err := dialHomeAndListen(LUSER, target, BindPort, reversePWD, reverseKey, retryMax)
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
	log.Printf("Dialling %s", target)
	_, err = net.DialTimeout("tcp", target, 5*time.Second)
	if err != nil {
		log.Println(err)
		return false
	}
	return true
}
