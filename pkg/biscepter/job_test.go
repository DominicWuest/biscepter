package biscepter

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetJobFromConfig(t *testing.T) {
	yml := `
repository: "repo"
goodCommit: "goodCommit"
badCommit: "badCommit"
buildCost: 42.25
ports:
  - 80
  - 443
healthcheck:
  - port: 1234
    type: http
    data: "/status"
dockerfile: "dockerfile"
`

	job, err := GetJobFromConfig(strings.NewReader(yml))
	assert.Nil(t, err, "GetJobFromConfig returned an error")

	assert.Equal(t, 42.25, job.BuildCost, "Mismatch in job field")
	assert.ElementsMatch(t, []int{80, 443}, job.Ports, "Mismatch in job field")
	assert.Equal(t, "goodCommit", job.GoodCommit, "Mismatch in job field")
	assert.Equal(t, "badCommit", job.BadCommit, "Mismatch in job field")
	assert.Equal(t, "dockerfile", job.Dockerfile, "Mismatch in job field")
	assert.Equal(t, "repo", job.Repository, "Mismatch in job field")
	assert.Equal(t, 1234, job.Healthchecks[0].Port, "Mismatch in job field")
	assert.Equal(t, HttpGet200, job.Healthchecks[0].CheckType, "Mismatch in job field")
	assert.Equal(t, "/status", job.Healthchecks[0].Data, "Mismatch in job field")
}

func TestGetDockerImageOfCommit(t *testing.T) {
	values := []struct {
		commit string
		hash   string
		image  string
	}{
		{"commit", "hash", "biscepter-commit:hash"},
		{"12345", "67890", "biscepter-12345:67890"},
		{"", "", "biscepter-:"},
	}

	for _, v := range values {
		job := Job{
			dockerfileHash: v.hash,
		}

		assert.Equal(t, v.image, job.getDockerImageOfCommit(v.commit), "Wrong docker image")
	}
}
