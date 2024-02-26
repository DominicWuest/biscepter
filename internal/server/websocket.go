package server

import "github.com/DominicWuest/biscepter/pkg/biscepter"

type websocketServer struct{}

func (*websocketServer) Init(int, chan biscepter.RunningSystem, chan biscepter.OffendingCommit) error {
	panic("unimplemented")
}
