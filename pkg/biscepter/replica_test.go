package biscepter

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestGetNextCommit(t *testing.T) {
	values := []struct {
		goodCommitOffset int
		badCommitOffset  int
		commits          []string
		built            []string
		buildCost        float64

		expectedIndex int
	}{
		{0, 6, []string{"padl", "a", "b", "c", "d", "e", "padr"}, []string{"c"}, 1e10, 3},
		{0, 6, []string{"padl", "a", "b", "c", "d", "e", "padr"}, []string{"b"}, 1e10, 2},
		{0, 6, []string{"padl", "a", "b", "c", "d", "e", "padr"}, []string{"d"}, 1e10, 4},
		{0, 6, []string{"padl", "a", "b", "c", "d", "e", "padr"}, []string{"a", "d"}, 1e10, 4},
		{0, 6, []string{"padl", "a", "b", "c", "d", "e", "padr"}, []string{"b", "e"}, 1e10, 2},
	}

	for i, v := range values {
		rep := replica{
			goodCommitOffset: v.goodCommitOffset,
			badCommitOffset:  v.badCommitOffset,
			commits:          v.commits,
			log:              logrus.NewEntry(logrus.StandardLogger()),
			parentJob: &Job{
				BuildCost:   v.buildCost,
				builtImages: make(map[string]bool),
			},
		}
		for _, image := range v.built {
			rep.parentJob.builtImages[rep.parentJob.getDockerImageOfCommit(image)] = true
		}

		logrus.SetLevel(logrus.TraceLevel)

		assert.Equalf(t, v.expectedIndex, rep.getNextCommit(), "GetNextCommit returned wrong offset for test %d; goodCommit: %d, badCommit: %d, commits: %v, built: %v, buildCost: %f", i, v.goodCommitOffset, v.badCommitOffset, v.commits, v.built, v.buildCost)
	}
}
