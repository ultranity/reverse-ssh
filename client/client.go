package client

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/user"
	"reverse-ssh/core"
	"strings"
	"time"

	gossh "golang.org/x/crypto/ssh"
)

var ActualPort string

func DialHomeAndListen(username string, address string, homeBindPort uint, reversePWD string, reverseKey string, retry_max int) (net.Listener, error) {
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
	ActualPort = strings.Split(ln.Addr().String(), ":")[1]

	//TODO: lnu, err := client.Listen("unix", "/tmp/ssh-reverse.sock") //必须为绝对路径
	log.Printf("Success: up on %s", ActualPort)

	// Attempt to send extra info back home in the info message of an extra ssh channel
	//TODO: sendExtraInfo(client, lnu.Addr().String())
	SendExtraInfo(client, ln.Addr().String())

	return ln, nil
}

func SendExtraInfo(client *gossh.Client, listeningAddress string) {

	extraInfo := core.ExtraInfo{ListeningAddress: listeningAddress}

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
	// The receiving end is expected to reject the channel, so "th4nkz" is a sign of success and we ignore it
	if err != nil && !strings.Contains(err.Error(), "th4nkz") {
		log.Printf("Could not create info channel: %+v", err)
	}

	// If the channel is actually accepted, just close it again
	if err == nil {
		go gossh.DiscardRequests(newReq)
		newChan.Close()
	}
}
