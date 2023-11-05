module reverse-ssh

go 1.21

require (
	github.com/ActiveState/termtest/conpty v0.5.0
	github.com/creack/pty v1.1.20
	github.com/desertbit/grumble v0.0.0-00010101000000-000000000000
	github.com/gliderlabs/ssh v0.3.5
	github.com/pkg/sftp v1.13.6
	golang.org/x/crypto v0.14.0
	golang.org/x/sys v0.14.0
)

replace github.com/desertbit/grumble => ./grumble

require (
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/anmitsu/go-shlex v0.0.0-20200514113438-38f4b401e2be // indirect
	github.com/desertbit/closer/v3 v3.5.2 // indirect
	github.com/desertbit/columnize v2.1.0+incompatible // indirect
	github.com/desertbit/go-shlex v0.1.1 // indirect
	github.com/desertbit/readline v1.5.1 // indirect
	github.com/fatih/color v1.15.0 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
)
