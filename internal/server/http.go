package server

import (
	"fmt"
	"net/http"

	"github.com/DominicWuest/biscepter/pkg/biscepter"
	"github.com/dchest/uniuri"
	"github.com/gin-gonic/gin"
)

type httpServer struct {
	rsChan chan biscepter.RunningSystem
	ocChan chan biscepter.OffendingCommit

	rsMap map[string]biscepter.RunningSystem
}

func (h *httpServer) Init(port int, rsChan chan biscepter.RunningSystem, ocChan chan biscepter.OffendingCommit) error {
	h.rsChan = rsChan
	h.ocChan = ocChan

	h.rsMap = make(map[string]biscepter.RunningSystem)

	router := gin.Default()

	router.GET("/system", h.getSystem)
	router.POST("/isGood/:systemId", h.postIsGood)
	router.POST("/isBad/:systemId", h.postIsBad)

	go router.Run(fmt.Sprintf("localhost:%d", port))
	return nil
}

type runningSystemResponse struct {
	SystemIndex string `json:"systemIndex"`

	ReplicaIndex int `json:"replicaIndex"`

	Ports map[int]int `json:"ports"`
}

type offendingCommitResponse struct {
	ReplicaIndex int `json:"replicaIndex"`

	Commit       string `json:"commit"`
	CommitOffset int    `json:"commitOffset"`

	CommitMessage string `json:"commitMessage"`
	CommitDate    string `json:"commitDate"`
	CommitAuthor  string `json:"commitAuthor"`
}

func (h *httpServer) getSystem(c *gin.Context) {
	select {
	case commit := <-h.ocChan:
		c.JSON(http.StatusOK, offendingCommitResponse{
			ReplicaIndex: commit.ReplicaIndex,

			Commit:       commit.Commit,
			CommitOffset: commit.CommitOffset,

			CommitMessage: commit.CommitMessage,
			CommitDate:    commit.CommitDate,
			CommitAuthor:  commit.CommitAuthor,
		})
	case system := <-h.rsChan:
		id := uniuri.New()
		h.rsMap[id] = system
		c.JSON(http.StatusOK, runningSystemResponse{
			SystemIndex: id,

			ReplicaIndex: system.ReplicaIndex,

			Ports: system.Ports,
		})
	}
}

func (h *httpServer) postIsGood(c *gin.Context) {
	id := c.Param("systemId")
	if rs, found := h.rsMap[id]; found {
		rs.IsGood()
		delete(h.rsMap, id)
		c.AbortWithStatus(200)
	} else {
		c.AbortWithStatus(404)
	}
}

func (h *httpServer) postIsBad(c *gin.Context) {
	id := c.Param("systemId")
	if rs, found := h.rsMap[id]; found {
		rs.IsBad()
		delete(h.rsMap, id)
		c.AbortWithStatus(200)
	} else {
		c.AbortWithStatus(404)
	}
}
