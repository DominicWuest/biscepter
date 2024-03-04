//go:build integration

package biscepter_test

import (
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/DominicWuest/biscepter/pkg/biscepter"
	"github.com/stretchr/testify/assert"
)

func bisectTestRepo(t *testing.T, replicas int, endpointOffset int, goodCommit, badCommit string, expectedCommits []string) {
	job := biscepter.Job{
		ReplicasCount: replicas,

		Ports: []int{3333},

		Healthchecks: []biscepter.Healthcheck{
			{Port: 3333, CheckType: biscepter.HttpGet200, Data: "/1", Config: biscepter.HealthcheckConfig{Retries: 30, Backoff: 2 * time.Second, MaxBackoff: 2 * time.Second}},
		},

		GoodCommit: goodCommit,
		BadCommit:  badCommit,

		Dockerfile: `
FROM golang:1.22.0-alpine
WORKDIR /app
COPY . .
CMD go run main.go
`,

		Repository: "git@github.com:DominicWuest/biscepter-test-repo.git",
	}

	// Running the job
	rsChan, ocChan, err := job.Run()
	if err != nil {
		panic(err)
	}

	// Waiting for running systems or offending commit
	offendingCommits := 0
	for {
		select {
		// Offending commit found
		case commit := <-ocChan:
			if commit.ReplicaIndex < 0 || commit.ReplicaIndex >= replicas {
				assert.FailNowf(t, "Failed to get offending commit", "Got bogus replica index: %d", commit.ReplicaIndex)
			}

			assert.Equal(t, expectedCommits[commit.ReplicaIndex], commit.Commit, "Bisection returned wrong commit")

			offendingCommits++
			if offendingCommits == replicas {
				err := job.Stop()
				assert.Nil(t, err, "Failed to stop job")
				return
			}
		case system := <-rsChan:
			res, err := http.Get(fmt.Sprintf("http://localhost:%d/%d", system.Ports[3333], system.ReplicaIndex+endpointOffset))
			assert.Nil(t, err, "Failed to get response from webserver")

			resBytes, err := io.ReadAll(res.Body)
			assert.Nil(t, err, "Failed to read response body")
			resText := string(resBytes)

			if resText == fmt.Sprint(system.ReplicaIndex+endpointOffset) {
				system.IsGood()
			} else {
				system.IsBad()
			}
		}
	}
}

func TestIntegration(t *testing.T) {
	t.Run("Bisecting Single Issue", func(t *testing.T) {
		bisectTestRepo(t,
			1,
			0,
			"8ee0e2a3c12e324c1b5c41f7861e341d91692efb",
			"9b70eda4f3e48d5d906f99b570a16d5a979b0a99",
			[]string{
				"03cdf844a180c44763e12f29901ab5f8d61444f3",
			},
		)
	})

	t.Run("Bisecting Multiple Issues", func(t *testing.T) {
		bisectTestRepo(t,
			3,
			0,
			"8ee0e2a3c12e324c1b5c41f7861e341d91692efb",
			"d3245c03595822db45d6cb990b417093ddc12af9",
			[]string{
				"03cdf844a180c44763e12f29901ab5f8d61444f3",
				"22a405d30a6c8d3eb045062ac2be4cff57e30d29",
				"9b70eda4f3e48d5d906f99b570a16d5a979b0a99",
			},
		)
	})

	t.Run("Bisecting Merges", func(t *testing.T) {
		bisectTestRepo(t,
			3,
			3,
			"76b5c32593cd9e9295db6c2e84bff32154427a65",
			"80afecdd27682647ffcd7a64483fbb207afdc675",
			[]string{
				"db9cf6aa3a666e41e69f50a783e59d57af724877",
				"72cad4a376c41aa6f83720d195c34cda83d6e7db",
				"cfad207f7deb9beb6855bc050d20d721945d30df",
			},
		)
	})
}
