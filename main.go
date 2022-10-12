package main

import (
	"NBDServer/nbdserver"
)

func main() {
	nbdsrv := nbdserver.New()
	nbdsrv.RunWithLoop("0.0.0.0:6666")
}
