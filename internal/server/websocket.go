package server

import "github.com/DominicWuest/biscepter/pkg/biscepter"

type websocketServer struct{}

func (*websocketServer) init(int, chan biscepter.RunningSystem, chan biscepter.OffendingCommit) error {
	panic("unimplemented")
}
