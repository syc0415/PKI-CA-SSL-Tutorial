package server

import (
	"simulation/conn"
	"testing"
)

/*
⣴⣦⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣴⡷
⠈⣿⣷⣦⣄⡀⠀⠀⠀⠀⠀⠀⠀⠀⣀⣤⣶⣿⣿
⠀⢸⣿⣿⣿⣿⣷⣆⣀⣀⣀⣀⣀⣾⣿⣿⣿⣿⡇
⠀⢸⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡇
⠀⠀⠿⢿⣿⣿⣿⣿⡏⡀⠀⡙⣿⣿⣿⣿⣿⠛
⠀⠀⠀⣿⣿⣿⡿⠟⠷⣅⣀⠵⠟⢿⣿⣿⣿⡆
⠀⠀⠀⣿⣿⠏⢲⣤⠀⠀⠀⠀⢠⣶⠙⣿⣿⠃
⠀⠀⠀⠘⢿⡄⠈⠃⠀⢐⢔⠀⠈⠋⢀⡿⠋
⠀⠀⠀⢀⢀⣼⣷⣶⣤⣤⣭⣤⣴⣶⣍
⠀⠀⠀⠈⠈⣈⢰⠿⠛⠉⠉⢻⢇⠆⣁⠁
⠀⠀⠀⠀⠀⠑⢸⠉⠀⠀⠀⠀⠁⡄⢘⣽⣿
⠀⠀⠀⠀⠀⠀⡜⠀⠀⢰⡆⠀⠀⠻⠛⠋
⠀⠀⠀⠀⠀⠀⠑⠒⠒⠈⠈⠒⠒⠊
||||||||||||||||||||||||||||||
||||||||||||KUROMI||||||||||||
|||||SERVER||||||||GOGOGO|||||
||||||||||||||||||||||||||||||
*/

func TestServer(t *testing.T) {
	var _ conn.ListenerInterface = conn.TCPListener
	var _ conn.HandlerInterface = conn.TCPHandler
	conn.TCPListener("../config/servercfg.yaml", conn.TCPHandler)
}
